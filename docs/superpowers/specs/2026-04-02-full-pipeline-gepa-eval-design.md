# Full-Pipeline GEPA Evaluation Design

## Problem

GEPA optimizes segmentation instructions by scoring segment boundary accuracy (F1) against gold labels. This ignores downstream translation quality — a segmentation that scores well on boundary F1 can produce segments that translate poorly (e.g., standalone 于 → "surname Yu").

Meanwhile, CEDICT lookup complexity (surname disambiguation, redirect resolution, entry ranking) has grown to compensate for segments the LLM could handle directly. This complexity belongs in the LLM, not in deterministic heuristics.

## Solution

Evaluate segmentation quality by scoring the full pipeline output: segment → translate → judge. Remove CEDICT from the translation path. Let the LLM handle pinyin and meaning directly via a single batched call per sentence.

## Production Pipeline Changes

### Remove from `DSPyProvider`

- `pinyinTranslator` and `meaningTranslator` fields + their dspy signatures
- `resolvePinyin`, `resolveMeaning`, `fallbackCedictPinyin` methods
- `SenseDisambiguator` interface and `SenseCandidate` struct from `provider.go`
- `senseSelector` field, `llmSenseSelector` struct, `resolveDictionarySense`
- `parseSelectedIndex`, `parseSelectedIndexFromResponse`
- `RankedEntries`, `PreferredEntry`, `rankedEntries`, `resolveRedirectEntry`
- `scoreEntryForDisplay`, `isSurnameOnlyDefinition`, `isRedirectOnlyDefinition`
- `extractRedirectTarget`, `definitionPartCount`, `cloneVisitedWords`
- `cedictRedirectPattern` regex
- `ComposeSegmentPinyin` (no longer needed — LLM handles all translation)
- `cedict` field from `DSPyProvider` struct
- `LookupCharacter` method and `LookupCharacter` from the `TranslationProvider` interface
- `IsCharAmbiguous`, `PreferredCharPinyin` — character-level CEDICT helpers
- `LookupFirst`, `Lookup`, `RankedEntries`, `PreferredEntry` — all CEDICT lookup methods
- `cedictDictionary` type, `loadCedictDictionary`, and all CEDICT infrastructure

### Remove CEDICT from Config

- Remove `CedictPath` from `config.Config` struct and env loading
- Remove `CEDICT_PATH` / `CEDIT_PATH` / `CCEDICT_PATH` env aliases
- Remove `CEDICT_PATH` from `.env.example`
- Update config tests (`TestLoadCedictPathAliases`)

### Remove CEDICT from Vocab/SRS Path

`ExtractAndLinkCharacters` in `store_vocab_srs.go` currently takes a `cedictLookup` func. Replace with segment data sourced from the translation results already in the DB.

- Remove `cedictLookup` parameter from `ExtractAndLinkCharacters`
- Remove `LookupCharacter` call in `handlers/vocab.go`
- New signature: `ExtractAndLinkCharacters(vocabItemID, headword string, charData []CharTranslation)` where `CharTranslation` is `{Char string, Pinyin string}`
- Caller builds `charData` from the segment results already stored in the DB — the segment's pinyin is space-delimited by syllable, split positionally to map each syllable to its character

### Keep

- `shouldSkipSegment` — guards non-CJK segments

### New Combined Translation Signature

One batched dspy call per sentence replaces the per-segment pinyin + meaning calls:

```
Inputs:
  segments_json:    JSON array of CJK segments to translate
  sentence:         The immediate sentence containing the segments
  full_text:        The complete input text for broader context

Output:
  translations_json: JSON array of {pinyin, english} objects, one per input segment
```

Hand-crafted instruction (fixed, not optimized):

> Given an array of Chinese word segments from a sentence, produce the pinyin (with tone marks) and a concise English translation for each segment. Use the sentence and full text for context to select the correct reading and meaning. Return a JSON array of objects with "pinyin" and "english" fields, in the same order as the input segments.

### Updated `TranslateSegments` Flow

```go
func (p *DSPyProvider) TranslateSegments(
    ctx context.Context,
    segments []string,
    sentence string,
    fullText string,
) ([]store.SegmentResult, error)
```

1. Partition segments: CJK segments (need translation) vs non-CJK (empty pinyin/english)
2. If no CJK segments, return all empty results
3. Single LLM call with CJK segments + sentence + full text
4. Parse `translations_json` response into `[]SegmentResult`
5. Zip CJK results back into position with non-CJK empty entries
6. Return combined results

### Interface Change

`TranslationProvider.TranslateSegments` signature changes — removes `LookupCharacter`, adds `fullText`:

```go
type TranslationProvider interface {
    Segment(ctx context.Context, text string) ([]string, error)
    TranslateSegments(ctx context.Context, segments []string, sentence string, fullText string) ([]store.SegmentResult, error)
    TranslateFullText(ctx context.Context, text string) (string, error)
}
```

Callers in `queue/manager.go` updated to pass full text through.

### TODO

Cap max sentence length to bound failure risk on long sentences. The batched call is more likely to produce malformed JSON or misaligned arrays with many segments. Add this if needed based on real-world failure rates.

## GEPA Harness Changes

### Full Pipeline Program

`NewFullPipelineProgram(workerLLM, segmentInstruction, translateInstruction)` creates a `core.Program` whose Forward function:

1. Segments the input sentence using the candidate segmentation instruction (worker LLM)
2. Filters CJK segments via `shouldSkipSegment`
3. Calls the batched translation signature (worker LLM) with segments + sentence + full-text context
4. Returns structured results:

```go
map[string]interface{}{
    "segments":          []string{...},
    "translations_json": string,  // JSON array of {text, pinyin, english}
    "text":              string,  // original sentence
    "paragraph":         string,  // full paragraph context
    "reconstruction_ok": bool,
    "parse_failed":      bool,
    "latency_ms":        float64,
}
```

No CEDICT dependency in the harness.

### Full Pipeline Metric

`fullPipelineMetric(judgeLLM)` returns a metric closure compatible with `gepa.Compile`:

```go
func fullPipelineMetric(judgeLLM core.LLM) func(expected, actual map[string]interface{}) float64
```

Scoring components:

1. **Reconstruction check** (hard constraint): segments concatenated must equal input text. Penalty: -0.45 if failed.
2. **Parse check**: penalty -0.35 if segment or translation parsing failed.
3. **Judge score** (replaces boundary F1): judge LLM evaluates translation quality, 0-10 normalized to 0.0-1.0.
4. **Latency penalty**: light penalty capped at 0.05 (same as current).

### Judge Prompt

```
Rate the quality of these Chinese word segmentations and translations.

Sentence: {sentence}
Paragraph context: {paragraph}

Segments:
1. 中华人民共和国 — Zhōnghuá Rénmín Gònghéguó — People's Republic of China
2. 成立 — chénglì — to establish
...

Score 0-10 as JSON: {"score": N}
Consider: Are word boundaries natural? Is each pinyin correct for this context?
Is each English meaning correct for this context?
```

Parsed via structured output. Normalized to 0.0-1.0.

### Judge LLM

- Same OpenAI-compatible completions API as worker
- Same `OPENAI_BASE_URL` and `OPENAI_API_KEY`
- Different model ID, passed via `--judge-model` CLI flag
- Instantiated via existing `NewSegmentationLLM(cfg, judgeModelID)`
- Falls back to worker model if `--judge-model` not specified

### Evaluation Flow (Per Paragraph)

1. Split paragraph into sentences (same `splitInputSentences` as production)
2. For each sentence:
   a. Segment (worker LLM with candidate instruction)
   b. Translate segments (worker LLM, with full_text as context)
   c. Judge scores sentence translations (judge LLM, with paragraph context)
3. Paragraph score = mean of sentence scores

### Updated `EvalSummary`

`ExactMatches` redefined: count of sentences where judge scored 10/10 (previously: count of sentences with exact segment boundary match).

All other fields (TotalCases, ReconstructionFail, Errors, TotalLatency) unchanged in meaning.

## Dataset Changes

### Format

```csv
id,paragraph
p01,"中华人民共和国成立于一九四九年。从那以后，中国经历了巨大的变化。"
p02,"如果你有时间，我们周末去爬山吧！那个地方很美。"
```

Two columns: `id` and `paragraph`. No gold segments. No gold translations. The judge scores everything.

### Content

Multi-sentence paragraphs reflecting real usage:
- Group existing 20 sentences into ~6-8 paragraphs of 2-4 sentences
- Add new paragraphs resembling actual user input: mixed register, names, dates, informal text
- Target: ~10-15 paragraphs

### Case struct update

```go
type Case struct {
    Name      string
    Paragraph string
}
```

`Text` and `Expected` fields replaced by `Paragraph`. No expected segments.

## CLI Changes

`cmd/gepa-segmentation/main.go`:

- New flag: `--judge-model` — OpenRouter model ID for the judge LLM (e.g. `anthropic/claude-sonnet-4`). Defaults to worker model.
- Remove: `--cedict-path` flag (CEDICT removed entirely).

## Unchanged

- `SplitCasesDeterministic` — splitting logic works the same, just on paragraphs instead of sentences
- `BuildConstrainedInstruction` — segmentation instruction templates unchanged
- `RunMultiSeedOptimization` — same multi-seed structure, updated to use full pipeline program and metric
- Promotion gating logic — same criteria (accuracy delta > 0, reconstruction not worse, errors not worse)
- Artifact writing — same files, same structure
- `splitInputSentences` in queue manager — production sentence splitting reused in eval
