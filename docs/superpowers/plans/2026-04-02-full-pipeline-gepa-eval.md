# Full-Pipeline GEPA Evaluation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Optimize segmentation instructions by evaluating full pipeline output (segment → translate → judge) instead of boundary F1, removing CEDICT from the translation path entirely.

**Architecture:** Remove CEDICT + per-segment LLM calls from `DSPyProvider`. Replace with one batched LLM call per sentence. Extend GEPA harness with full pipeline evaluation using a judge LLM. New paragraph-based dataset replaces sentence-level gold labels.

**Tech Stack:** Go, dspy-go, OpenAI-compatible API (OpenRouter), SQLite

**Spec:** `docs/superpowers/specs/2026-04-02-full-pipeline-gepa-eval-design.md`

---

## File Map

### Modified Files

| File | Responsibility |
|---|---|
| `server/internal/intelligence/provider.go` | `TranslationProvider` interface — remove `LookupCharacter`, change `TranslateSegments` signature |
| `server/internal/intelligence/translation/dspy_provider.go` | Remove CEDICT, per-segment calls; add batched translation signature |
| `server/internal/intelligence/translation/cedict.go` | Delete entirely |
| `server/internal/intelligence/translation/cedict_test.go` | Delete entirely |
| `server/internal/intelligence/translation/dspy_provider_cedict_test.go` | Delete entirely (untracked) |
| `server/internal/queue/manager.go` | Update `TranslateSegments` calls — batch per sentence, pass `fullText` |
| `server/internal/queue/manager_test.go` | Update mock provider — new signature, remove `LookupCharacter` |
| `server/internal/http/handlers/deps.go` | Update `srsStore` interface — `ExtractAndLinkCharacters` new signature |
| `server/internal/http/handlers/translation.go` | Update `TranslateBatch` handler — new signature |
| `server/internal/http/handlers/vocab.go` | Remove `LookupCharacter` call, pass `charData` to `ExtractAndLinkCharacters` |
| `server/internal/translation/store_vocab_srs.go` | Update `ExtractAndLinkCharacters` — new signature, remove CEDICT lookup |
| `server/internal/config/config.go` | Remove `CedictPath` field and env loading |
| `server/internal/config/config_test.go` | Remove CEDICT config tests |
| `server/tests/integration/helpers_test.go` | Remove `CedictPath` from test config |
| `server/tests/integration/chat_rest_test.go` | Update mock — new signature, remove `LookupCharacter` |
| `server/tests/integration/upstream_llm_test.go` | Update to use new `TranslateSegments` signature |
| `server/.env.example` | Remove `CEDICT_PATH` |
| `server/scripts-go/segmentation/gepa_harness.go` | New full pipeline program, metric, judge; update dataset loading |
| `server/cmd/gepa-segmentation/main.go` | Add `--judge-model` flag, remove `--cedict-path` |
| `data/jepa/datasets/paragraphs.csv` | New paragraph dataset (replaces `sentences_20.csv`) |

---

## Task 1: Revert Uncommitted CEDICT Complexity

Discard the uncommitted changes that added `SenseDisambiguator`, `RankedEntries`, `PreferredEntry`, redirect resolution, and scoring heuristics. Start from a clean baseline.

**Files:**
- Revert: `server/internal/intelligence/provider.go`
- Revert: `server/internal/intelligence/translation/cedict.go`
- Revert: `server/internal/intelligence/translation/cedict_test.go`
- Revert: `server/internal/intelligence/translation/dspy_provider.go`
- Revert: `server/tests/integration/upstream_llm_test.go`
- Delete: `server/internal/intelligence/translation/dspy_provider_cedict_test.go`

- [ ] **Step 1: Revert tracked files to HEAD**

```bash
cd server
git checkout HEAD -- \
  internal/intelligence/provider.go \
  internal/intelligence/translation/cedict.go \
  internal/intelligence/translation/cedict_test.go \
  internal/intelligence/translation/dspy_provider.go \
  tests/integration/upstream_llm_test.go
```

- [ ] **Step 2: Remove untracked test file**

```bash
rm server/internal/intelligence/translation/dspy_provider_cedict_test.go
```

- [ ] **Step 3: Verify tests pass on clean baseline**

```bash
cd server && go test ./internal/intelligence/translation/... -v -count=1
```

Expected: All existing tests pass (cedict_test.go, parse_test.go).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "revert: remove uncommitted CEDICT disambiguation complexity

Clean baseline before pipeline rework. Reverts SenseDisambiguator,
RankedEntries, PreferredEntry, redirect resolution, and scoring heuristics."
```

---

## Task 2: Update `TranslationProvider` Interface

Change the interface to remove `LookupCharacter` and update `TranslateSegments` to accept `sentence` and `fullText` instead of `sentenceContext`.

**Files:**
- Modify: `server/internal/intelligence/provider.go`

- [ ] **Step 1: Update the interface**

In `server/internal/intelligence/provider.go`, replace the `TranslationProvider` interface:

```go
type TranslationProvider interface {
	Segment(ctx context.Context, text string) ([]string, error)
	TranslateSegments(ctx context.Context, segments []string, sentence string, fullText string) ([]translation.SegmentResult, error)
	TranslateFull(ctx context.Context, text string) (string, error)
}
```

Remove the `SenseCandidate` struct and `SenseDisambiguator` interface (if still present after revert — they were in the committed code at HEAD, verify first).

- [ ] **Step 2: Verify the file compiles in isolation**

```bash
cd server && go vet ./internal/intelligence/...
```

Expected: Compile errors in downstream files (manager.go, dspy_provider.go, handlers, tests) — that's correct, we'll fix those in subsequent tasks.

- [ ] **Step 3: Commit**

```bash
git add server/internal/intelligence/provider.go
git commit -m "refactor: update TranslationProvider interface

Remove LookupCharacter. Change TranslateSegments signature to accept
sentence and fullText for batched translation context."
```

---

## Task 3: Remove CEDICT Infrastructure

Delete `cedict.go`, `cedict_test.go`, and all CEDICT references from `DSPyProvider` and config.

**Files:**
- Delete: `server/internal/intelligence/translation/cedict.go`
- Delete: `server/internal/intelligence/translation/cedict_test.go`
- Modify: `server/internal/intelligence/translation/dspy_provider.go`
- Modify: `server/internal/config/config.go`
- Modify: `server/internal/config/config_test.go`
- Modify: `server/.env.example`

- [ ] **Step 1: Delete CEDICT files**

```bash
rm server/internal/intelligence/translation/cedict.go
rm server/internal/intelligence/translation/cedict_test.go
```

- [ ] **Step 2: Remove CEDICT from DSPyProvider**

In `server/internal/intelligence/translation/dspy_provider.go`:

Remove from the `DSPyProvider` struct:
- `cedict *cedictDictionary` field

Remove from `NewDSPyProvider`:
- `loadCedictDictionary` call and the `cedict` field assignment
- The `cfg.CedictPath` reference
- The `cedict` log warning

Remove these methods entirely:
- `ComposeSegmentPinyin`
- `LookupCharacter`
- `resolvePinyin` (entire method)
- `resolveMeaning` (entire method)
- `fallbackCedictPinyin`

Remove these fields from the struct:
- `pinyinTranslator *modules.Predict`
- `meaningTranslator *modules.Predict`

Remove from `NewDSPyProvider`:
- `pinyinSig` signature definition and `pinyinTranslator` module creation
- `meaningSig` signature definition and `meaningTranslator` module creation
- Their assignments in the return struct

Keep `segmenter` and `fullTranslator` — they're still needed.

Leave `TranslateSegments` as a stub that returns an error for now — we'll implement the new version in Task 4.

```go
func (p *DSPyProvider) TranslateSegments(ctx context.Context, segments []string, sentence string, fullText string) ([]store.SegmentResult, error) {
	return nil, fmt.Errorf("TranslateSegments: not yet reimplemented")
}
```

- [ ] **Step 3: Remove CEDICT from config**

In `server/internal/config/config.go`:
- Remove `CedictPath string` from the `Config` struct
- Remove the `CedictPath` line from the `Load()` function (the `envFirstOrDefault` call with `CEDICT_PATH`, `CEDIT_PATH`, `CCEDICT_PATH`)

In `server/internal/config/config_test.go`:
- Remove the `cfg.CedictPath` assertion from the main config test
- Remove the entire `TestLoadCedictPathAliases` function

In `server/.env.example`:
- Remove the `CEDICT_PATH=data/cedict_ts.u8` line

- [ ] **Step 4: Remove CEDICT from `candidateCompiledInstructionPaths`**

In `server/internal/intelligence/translation/dspy_provider.go`, the `candidateCompiledInstructionPaths` function references `cfg.CedictPath`. Update it to remove that branch:

```go
func candidateCompiledInstructionPaths(cfg config.Config) []string {
	return []string{
		filepath.Join("server", "data", "jepa", "compiled_instruction.txt"),
		filepath.Join("data", "jepa", "compiled_instruction.txt"),
	}
}
```

- [ ] **Step 5: Verify compilation**

```bash
cd server && go vet ./internal/intelligence/... ./internal/config/...
```

Expected: May still have errors in downstream callers (manager, handlers, tests) — that's OK.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove CEDICT infrastructure

Delete cedict.go, cedict_test.go. Remove dictionary from DSPyProvider,
config, and env. Translation now handled entirely by LLM."
```

---

## Task 4: Implement Batched `TranslateSegments`

Replace the stub with the new batched implementation — one dspy call per sentence.

**Files:**
- Modify: `server/internal/intelligence/translation/dspy_provider.go`

- [ ] **Step 1: Add the batched translation signature to `NewDSPyProvider`**

Add after the `fullTranslateSig`:

```go
batchTranslateSig := core.NewSignature(
	[]core.InputField{
		{Field: core.NewField("segments_json", core.WithDescription("JSON array of Chinese segments to translate"))},
		{Field: core.NewField("sentence", core.WithDescription("The sentence containing the segments"))},
		{Field: core.NewField("full_text", core.WithDescription("The complete input text for broader context"))},
	},
	[]core.OutputField{
		{Field: core.NewField("translations_json", core.WithDescription("JSON array of {pinyin, english} objects in same order as input segments"))},
	},
).WithInstruction("Given an array of Chinese word segments from a sentence, produce the pinyin (with tone marks) and a concise English translation for each segment. Use the sentence and full text for context to select the correct reading and meaning. Return a JSON array of objects with \"pinyin\" and \"english\" fields, in the same order as the input segments.")

batchTranslator := modules.NewPredict(batchTranslateSig).WithStructuredOutput()
batchTranslator.SetLLM(openAILLM)
```

Add `batchTranslator *modules.Predict` to the `DSPyProvider` struct and assign it in the return.

- [ ] **Step 2: Implement `TranslateSegments`**

```go
func (p *DSPyProvider) TranslateSegments(ctx context.Context, segments []string, sentence string, fullText string) ([]store.SegmentResult, error) {
	type indexedSegment struct {
		originalIdx int
		segment     string
	}

	out := make([]store.SegmentResult, len(segments))
	var cjkSegments []indexedSegment

	for i, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" || shouldSkipSegment(seg) {
			out[i] = store.SegmentResult{Segment: seg}
			continue
		}
		cjkSegments = append(cjkSegments, indexedSegment{originalIdx: i, segment: seg})
	}

	if len(cjkSegments) == 0 {
		return out, nil
	}

	segStrings := make([]string, len(cjkSegments))
	for i, cs := range cjkSegments {
		segStrings[i] = cs.segment
	}

	segJSON, err := json.Marshal(segStrings)
	if err != nil {
		return nil, fmt.Errorf("marshal segments: %w", err)
	}

	res, err := p.batchTranslator.Process(ctx, map[string]any{
		"segments_json": string(segJSON),
		"sentence":      sentence,
		"full_text":     fullText,
	})
	if err != nil {
		return nil, fmt.Errorf("batch translate: %w", err)
	}

	translations := parseBatchTranslations(res["translations_json"])
	if len(translations) == 0 {
		translations = parseBatchTranslations(res["response"])
	}

	for i, cs := range cjkSegments {
		result := store.SegmentResult{Segment: cs.segment}
		if i < len(translations) {
			result.Pinyin = translations[i].Pinyin
			result.English = translations[i].English
		}
		out[cs.originalIdx] = result
	}

	return out, nil
}
```

- [ ] **Step 3: Add `parseBatchTranslations` helper**

```go
type batchTranslation struct {
	Pinyin  string `json:"pinyin"`
	English string `json:"english"`
}

func parseBatchTranslations(v any) []batchTranslation {
	if v == nil {
		return nil
	}
	var raw string
	switch t := v.(type) {
	case string:
		raw = t
	case []any:
		b, err := json.Marshal(t)
		if err != nil {
			return nil
		}
		raw = string(b)
	default:
		raw = normalizeJSONLikePayload(strings.TrimSpace(toString(v)))
	}
	raw = normalizeJSONLikePayload(strings.TrimSpace(raw))
	if raw == "" {
		return nil
	}
	var translations []batchTranslation
	if err := json.Unmarshal([]byte(raw), &translations); err != nil {
		// Try extracting JSON array from freeform text.
		if arr := extractJSONArray(raw); len(arr) > 0 {
			b, _ := json.Marshal(arr)
			_ = json.Unmarshal(b, &translations)
		}
	}
	return translations
}
```

Note: `extractJSONArray`, `normalizeJSONLikePayload`, and `toString` already exist in `parse.go`.

- [ ] **Step 4: Verify compilation**

```bash
cd server && go vet ./internal/intelligence/...
```

Expected: Passes. Downstream callers still broken (next tasks).

- [ ] **Step 5: Commit**

```bash
git add server/internal/intelligence/translation/dspy_provider.go
git commit -m "feat: implement batched TranslateSegments

One dspy call per sentence with segments_json, sentence, and full_text
context. Replaces per-segment pinyin + meaning calls."
```

---

## Task 5: Update Queue Manager

Change both `runJob` and `StartReprocessing` to batch segments per sentence and pass `fullText`.

**Files:**
- Modify: `server/internal/queue/manager.go`

- [ ] **Step 1: Update `runJob` to batch per sentence**

The current code loops over individual `queuedSegment` entries and calls `TranslateSegments` with a single segment each. Change to batch all segments for a sentence in one call.

Replace the loop at line ~380 (`for idx := startIndex; idx < len(queued); idx++`) with:

```go
// Group queued segments by sentence index for batched translation.
type sentenceBatch struct {
	sentenceIdx  int
	sentenceText string
	segments     []string
	startIdx     int // index into queued for progress tracking
}

var batches []sentenceBatch
var currentBatch *sentenceBatch
for idx := startIndex; idx < len(queued); idx++ {
	work := queued[idx]
	if currentBatch == nil || currentBatch.sentenceIdx != work.SentenceIndex {
		if currentBatch != nil {
			batches = append(batches, *currentBatch)
		}
		currentBatch = &sentenceBatch{
			sentenceIdx:  work.SentenceIndex,
			sentenceText: work.SentenceText,
			startIdx:     idx,
		}
	}
	currentBatch.segments = append(currentBatch.segments, work.Segment)
}
if currentBatch != nil {
	batches = append(batches, *currentBatch)
}

for _, batch := range batches {
	translated, err := m.provider.TranslateSegments(ctx, batch.segments, batch.sentenceText, item.InputText)
	if err != nil || len(translated) == 0 {
		_ = m.store.Fail(translationID, "Failed to translate segments")
		return
	}
	for i, segmentResult := range translated {
		if _, _, err := m.store.AddProgressSegment(translationID, segmentResult, batch.sentenceIdx); err != nil {
			_ = m.store.Fail(translationID, "Failed to update translation progress")
			return
		}
		_ = i // progress is tracked by AddProgressSegment
	}
}
```

Remove the `time.Sleep(15 * time.Millisecond)` — no longer needed since we batch.

- [ ] **Step 2: Update `StartReprocessing` similarly**

In the reprocessing goroutine (around line ~254), change the per-segment loop to batch per sentence. The pattern is the same — group `allWork` by `sentenceIdx`, batch call, then store with explicit indices:

```go
// Group by sentence for batched translation.
type reprocessBatch struct {
	sentenceIdx  int
	sentenceText string
	segments     []string
}
batchMap := make(map[int]*reprocessBatch)
for _, work := range allWork {
	b, ok := batchMap[work.sentenceIdx]
	if !ok {
		b = &reprocessBatch{sentenceIdx: work.sentenceIdx, sentenceText: work.sentenceText}
		batchMap[work.sentenceIdx] = b
	}
	b.segments = append(b.segments, work.segment)
}

for _, sentenceIdx := range orderedIdxs {
	b, ok := batchMap[sentenceIdx]
	if !ok {
		continue
	}
	translated, err := m.provider.TranslateSegments(ctx, b.segments, b.sentenceText, item.InputText)
	if err != nil || len(translated) == 0 {
		_ = m.store.Fail(translationID, "Failed to translate segment during reprocessing")
		return
	}
	for segIdx, result := range translated {
		if err := m.store.AddReprocessedSegment(translationID, result, sentenceIdx, segIdx); err != nil {
			_ = m.store.Fail(translationID, "Failed to store reprocessed segment")
			return
		}
	}
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd server && go vet ./internal/queue/...
```

Expected: Compile error in `manager_test.go` (mock has old signature) — fixed in next task.

- [ ] **Step 4: Commit**

```bash
git add server/internal/queue/manager.go
git commit -m "refactor: batch TranslateSegments calls per sentence in queue manager

Group segments by sentence and make one batched call per sentence,
passing full input text as context."
```

---

## Task 6: Update All Mocks and Callers

Fix all remaining compile errors — mocks, handlers, integration tests.

**Files:**
- Modify: `server/internal/queue/manager_test.go`
- Modify: `server/tests/integration/chat_rest_test.go`
- Modify: `server/tests/integration/helpers_test.go`
- Modify: `server/internal/http/handlers/translation.go`
- Modify: `server/internal/http/handlers/vocab.go`
- Modify: `server/internal/http/handlers/deps.go`
- Modify: `server/internal/translation/store_vocab_srs.go`

- [ ] **Step 1: Update mock in `manager_test.go`**

Remove `LookupCharacter` method. Update `TranslateSegments` signature:

```go
func (m *mockProvider) TranslateSegments(_ context.Context, segments []string, _ string, _ string) ([]translation.SegmentResult, error) {
	out := make([]translation.SegmentResult, 0, len(segments))
	for _, seg := range segments {
		out = append(out, translation.SegmentResult{
			Segment: seg,
			Pinyin:  "mock-pinyin",
			English: "mock-english",
		})
	}
	return out, nil
}
```

- [ ] **Step 2: Update mock in `chat_rest_test.go`**

Remove `LookupCharacter` method. Update `TranslateSegments` signature:

```go
func (m mockTranslationProvider) TranslateSegments(_ context.Context, segments []string, _ string, _ string) ([]translation.SegmentResult, error) {
	out := make([]translation.SegmentResult, 0, len(segments))
	for _, seg := range segments {
		out = append(out, translation.SegmentResult{
			Segment: seg,
			Pinyin:  "mock",
			English: "mock",
		})
	}
	return out, nil
}
```

- [ ] **Step 3: Remove `CedictPath` from integration test helpers**

In `server/tests/integration/helpers_test.go`:
- Remove `CedictPath` from both config structs (the default one and the upstream one)
- Remove the `cedictPath` variable and its env lookup logic

- [ ] **Step 4: Update `TranslateBatch` handler**

In `server/internal/http/handlers/translation.go`, update the call:

```go
segmentResults, err := transProvider.TranslateSegments(context.Background(), req.Segments, derefOr(req.Context, ""), derefOr(req.Context, ""))
```

The batch endpoint doesn't have a separate fullText — use context for both sentence and fullText.

- [ ] **Step 5: Update `ExtractAndLinkCharacters` signature**

In `server/internal/translation/store_vocab_srs.go`, change the signature:

```go
type CharTranslation struct {
	Char   string
	Pinyin string
}

func (s *SRSStore) ExtractAndLinkCharacters(vocabItemID string, headword string, charData []CharTranslation) error {
	runes := []rune(headword)
	cjkCount := 0
	for _, r := range runes {
		if isCJKIdeograph(r) {
			cjkCount++
		}
	}
	if cjkCount <= 1 {
		return nil
	}

	// Build a map from character to pinyin from the provided data.
	charPinyinMap := make(map[string]string, len(charData))
	for _, cd := range charData {
		charPinyinMap[cd.Char] = cd.Pinyin
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	seen := make(map[string]bool)
	for _, r := range runes {
		if !isCJKIdeograph(r) {
			continue
		}
		char := string(r)
		pinyin := charPinyinMap[char]
		dedupKey := char + "|" + pinyin
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true

		charID, _ := newID()
		_, _ = s.db.Exec(
			`INSERT OR IGNORE INTO vocab_items (id, headword, pinyin, english, type, status, created_at, updated_at)
			 VALUES (?, ?, ?, '', 'character', 'learning', ?, ?)`,
			charID, char, pinyin, now, now,
		)

		var resolvedCharID string
		if err := s.db.QueryRow(
			`SELECT id FROM vocab_items WHERE headword = ? AND type = 'character'`, char,
		).Scan(&resolvedCharID); err != nil {
			continue
		}

		_, _ = s.db.Exec(
			`INSERT OR IGNORE INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
			 VALUES (?, ?, 0, 2.5, 0, 0, ?)`,
			resolvedCharID, now, now,
		)

		linkID, _ := newID()
		_, _ = s.db.Exec(
			`INSERT OR IGNORE INTO character_word_links (id, character_item_id, word_item_id, created_at)
			 VALUES (?, ?, ?, ?)`,
			linkID, resolvedCharID, vocabItemID, now,
		)
	}
	return nil
}
```

- [ ] **Step 6: Update `srsStore` interface and vocab handler**

In `server/internal/http/handlers/deps.go`, update the interface method:

```go
ExtractAndLinkCharacters(vocabItemID string, headword string, charData []translation.CharTranslation) error
```

Note: `CharTranslation` needs to be in the `translation` package since the interface references it. Move the struct definition to `server/internal/translation/store.go`:

```go
type CharTranslation struct {
	Char   string
	Pinyin string
}
```

And remove it from `store_vocab_srs.go`.

In `server/internal/http/handlers/vocab.go`, replace the `LookupCharacter` call:

```go
// Build per-character pinyin from segment pinyin.
// For now, pass empty charData — full implementation comes with data model spec.
_ = srs.ExtractAndLinkCharacters(id, req.Headword, nil)
```

- [ ] **Step 7: Run full build**

```bash
cd server && go build ./...
```

Expected: Compiles cleanly.

- [ ] **Step 8: Run all tests**

```bash
cd server && go test ./... -count=1
```

Expected: All tests pass (except integration tests needing DB/env — those are expected to skip).

- [ ] **Step 9: Commit**

```bash
cd server && gofmt -w .
git add -A
git commit -m "refactor: update all callers for new TranslationProvider interface

Update mocks, handlers, queue manager tests, integration test helpers.
Remove CEDICT from config, ExtractAndLinkCharacters, and .env.example."
```

---

## Task 7: Write Unit Tests for Batched `TranslateSegments`

**Files:**
- Create: `server/internal/intelligence/translation/translate_segments_test.go`

- [ ] **Step 1: Write test for non-CJK segments skipped**

```go
package translation

import (
	"context"
	"testing"
)

func TestTranslateSegments_SkipsNonCJK(t *testing.T) {
	t.Parallel()
	provider := &DSPyProvider{}
	results, err := provider.TranslateSegments(context.Background(), []string{"。", "!", " "}, "test", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Pinyin != "" || r.English != "" {
			t.Fatalf("result[%d] should have empty pinyin/english, got pinyin=%q english=%q", i, r.Pinyin, r.English)
		}
	}
}
```

- [ ] **Step 2: Run test**

```bash
cd server && go test ./internal/intelligence/translation/ -run TestTranslateSegments_SkipsNonCJK -v
```

Expected: PASS.

- [ ] **Step 3: Write test for `parseBatchTranslations`**

```go
func TestParseBatchTranslations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  int
	}{
		{"nil", nil, 0},
		{"empty string", "", 0},
		{"valid json array", `[{"pinyin":"nǐ hǎo","english":"hello"},{"pinyin":"shì jiè","english":"world"}]`, 2},
		{"json in markdown fence", "```json\n[{\"pinyin\":\"nǐ\",\"english\":\"you\"}]\n```", 1},
		{"nested response", map[string]any{"translations_json": `[{"pinyin":"hǎo","english":"good"}]`}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBatchTranslations(tt.input)
			if len(got) != tt.want {
				t.Fatalf("parseBatchTranslations(%v) returned %d items, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}
```

- [ ] **Step 4: Run test**

```bash
cd server && go test ./internal/intelligence/translation/ -run TestParseBatchTranslations -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/intelligence/translation/translate_segments_test.go
git commit -m "test: add unit tests for batched TranslateSegments"
```

---

## Task 8: Update GEPA Dataset to Paragraphs

Replace `sentences_20.csv` with `paragraphs.csv`. Update the `Case` struct and CSV loader.

**Files:**
- Create: `data/jepa/datasets/paragraphs.csv`
- Modify: `server/scripts-go/segmentation/gepa_harness.go`

- [ ] **Step 1: Create paragraph dataset**

```csv
id,paragraph
p01,我喜欢学习中文。人工智能改变世界。
p02,今天下午我们一起去图书馆看书。亡羊补牢，为时未晚。
p03,中华人民共和国成立于一九四九年。研究生命起源。
p04,你今天怎么这么开心呀？请把这份文件发到我的邮箱。
p05,虽然天气很冷，但是大家都按时到了。这个问题看起来简单，其实不容易回答。
p06,我们计划在下个月上线新的支付系统。如果你有时间，我们周末去爬山吧！
p07,这部电影我已经看过三遍了。老师要求我们明天之前提交作业。
p08,上海和北京都是中国的重要城市。为了提高效率，团队决定优化部署流程。
p09,她一边听音乐，一边整理房间。请确认订单号A12345是否已经付款。
p10,系统在高并发场景下仍然保持稳定运行。经过多轮讨论，方案终于达成一致意见。
p11,这座城市的建筑风格非常独特。从古至今，人们一直在追求更好的生活方式。
p12,小王昨天在咖啡店遇到了老朋友，聊了很久关于工作和家庭的事情。他们约好下周末一起去爬山。
```

- [ ] **Step 2: Update `Case` struct and CSV loader**

In `server/scripts-go/segmentation/gepa_harness.go`:

Replace the `Case` struct:

```go
type Case struct {
	Name      string
	Paragraph string
}
```

Update `LoadCasesFromCSV` — change column expectations from `(id, sentence, expected_segments_json)` to `(id, paragraph)`:

```go
const (
	csvHeaderID        = "id"
	csvHeaderParagraph = "paragraph"
)
```

Update `csvColumnIndices`:

```go
func csvColumnIndices(header []string) (map[string]int, error) {
	indices := map[string]int{}
	for i, col := range header {
		indices[strings.TrimSpace(strings.ToLower(col))] = i
	}
	required := []string{csvHeaderID, csvHeaderParagraph}
	for _, k := range required {
		if _, ok := indices[k]; !ok {
			return nil, fmt.Errorf("csv header missing required column %q", k)
		}
	}
	return indices, nil
}
```

Update `LoadCasesFromCSV` to read paragraph instead of sentence + expected segments:

```go
func LoadCasesFromCSV(path string) ([]Case, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("csv has no data rows")
	}

	indices, err := csvColumnIndices(records[0])
	if err != nil {
		return nil, err
	}

	idIdx := indices[csvHeaderID]
	paraIdx := indices[csvHeaderParagraph]
	var cases []Case
	for _, row := range records[1:] {
		if len(row) <= paraIdx || len(row) <= idIdx {
			continue
		}
		paragraph := strings.TrimSpace(row[paraIdx])
		if paragraph == "" {
			continue
		}
		cases = append(cases, Case{
			Name:      strings.TrimSpace(row[idIdx]),
			Paragraph: paragraph,
		})
	}
	return cases, nil
}
```

Remove the old constants: `csvHeaderSentence`, `csvHeaderExpectedSegJSON`, `minCSVRowFieldCount`.

- [ ] **Step 3: Update `BuildGEPASentenceDataset`**

Rename to `BuildGEPAParagraphDataset` and update to use paragraph:

```go
func BuildGEPAParagraphDataset(corpus []Case, maxUnits int) (*datasets.SimpleDataset, []core.Example) {
	examples := make([]core.Example, 0, maxUnits)
	for _, tc := range corpus {
		paragraph := strings.TrimSpace(tc.Paragraph)
		if paragraph == "" {
			continue
		}
		examples = append(examples, core.Example{
			Inputs:  map[string]interface{}{"paragraph": paragraph},
			Outputs: map[string]interface{}{"paragraph": paragraph},
		})
		if len(examples) >= maxUnits {
			return datasets.NewSimpleDataset(examples), examples
		}
	}
	return datasets.NewSimpleDataset(examples), examples
}
```

- [ ] **Step 4: Update default CSV path constant**

```go
const DefaultCSVPath = "data/jepa/datasets/paragraphs.csv"
```

- [ ] **Step 5: Verify compilation**

```bash
cd server && go vet ./scripts-go/segmentation/...
```

Expected: Compile errors in functions that reference old `Case.Text` and `Case.Expected` — fixed in next task.

- [ ] **Step 6: Commit**

```bash
git add data/jepa/datasets/paragraphs.csv server/scripts-go/segmentation/gepa_harness.go
git commit -m "refactor: switch GEPA dataset from sentences to paragraphs

New paragraphs.csv with multi-sentence entries. Update Case struct,
CSV loader, and dataset builder."
```

---

## Task 9: Implement Full Pipeline Program and Metric

The core of the GEPA rework — `NewFullPipelineProgram` and `fullPipelineMetric`.

**Files:**
- Modify: `server/scripts-go/segmentation/gepa_harness.go`

- [ ] **Step 1: Add `splitInputSentences` import or copy**

The queue manager's `splitInputSentences` is in an internal package. The simplest approach: extract it to a shared utility, or copy the function into the segmentation package. Since `splitInputSentences` is tightly coupled to the queue's `sentenceInfo` type, copy the minimal sentence-splitting logic:

```go
// splitParagraphSentences splits a paragraph into sentences for evaluation.
// Mirrors the production splitInputSentences logic from queue/manager.go.
func splitParagraphSentences(text string) []string {
	var out []string
	var current strings.Builder
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]
		if isSentenceEnd(r) {
			s := strings.TrimSpace(current.String())
			if s != "" {
				out = append(out, s)
			}
			current.Reset()
			continue
		}
		if r == '\n' || r == '\r' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				out = append(out, s)
			}
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	s := strings.TrimSpace(current.String())
	if s != "" {
		out = append(out, s)
	}
	return out
}

func isSentenceEnd(r rune) bool {
	return r == '。' || r == '！' || r == '？' || r == '!' || r == '?'
}
```

- [ ] **Step 2: Implement `NewFullPipelineProgram`**

```go
func NewFullPipelineProgram(workerLLM core.LLM, segmentInstruction string, translateInstruction string) core.Program {
	segmenter := &stickyPredict{Predict: modules.NewPredict(buildSegmentSignature(segmentInstruction)).WithStructuredOutput()}
	segmenter.SetLLM(workerLLM)

	translateSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("segments_json", core.WithDescription("JSON array of Chinese segments to translate"))},
			{Field: core.NewField("sentence", core.WithDescription("The sentence containing the segments"))},
			{Field: core.NewField("full_text", core.WithDescription("The complete input text for broader context"))},
		},
		[]core.OutputField{
			{Field: core.NewField("translations_json", core.WithDescription("JSON array of {pinyin, english} objects in same order as input segments"))},
		},
	).WithInstruction(translateInstruction)

	translator := modules.NewPredict(translateSig).WithStructuredOutput()
	translator.SetLLM(workerLLM)

	return core.Program{
		Modules: map[string]core.Module{"segmenter": segmenter},
		Forward: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			paragraph, _ := inputs["paragraph"].(string)
			paragraph = strings.TrimSpace(paragraph)
			start := time.Now()

			sentences := splitParagraphSentences(paragraph)
			if len(sentences) == 0 {
				return map[string]interface{}{
					"paragraph":      paragraph,
					"parse_failed":   true,
					"latency_ms":     float64(time.Since(start).Milliseconds()),
				}, nil
			}

			type sentenceResult struct {
				sentence     string
				segments     []string
				translations string // JSON
				reconOK      bool
				parseFailed  bool
			}

			var results []sentenceResult
			allParseFailed := false

			for _, sent := range sentences {
				callCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
				res, err := segmenter.Process(callCtx, map[string]any{"text": sent})
				cancel()
				if err != nil {
					allParseFailed = true
					results = append(results, sentenceResult{sentence: sent, parseFailed: true})
					continue
				}

				segments := parseSegments(res["segments"])
				if len(segments) == 0 {
					segments = parseSegmentsFromResponse(res["response"])
				}
				if len(segments) == 0 {
					segments = parseLooseSegments(toString(res["segments"]))
				}
				parseFailed := len(segments) == 0

				reconOK := normalizeForReconstruction(strings.Join(segments, "")) == normalizeForReconstruction(sent)

				// Filter CJK segments for translation.
				var cjkSegments []string
				for _, seg := range segments {
					seg = strings.TrimSpace(seg)
					if seg != "" && !shouldSkipSegment(seg) {
						cjkSegments = append(cjkSegments, seg)
					}
				}

				translationsJSON := "[]"
				if len(cjkSegments) > 0 {
					segJSON, _ := json.Marshal(cjkSegments)
					tCtx, tCancel := context.WithTimeout(ctx, 40*time.Second)
					tRes, tErr := translator.Process(tCtx, map[string]any{
						"segments_json": string(segJSON),
						"sentence":      sent,
						"full_text":     paragraph,
					})
					tCancel()
					if tErr == nil {
						raw := toString(tRes["translations_json"])
						if raw == "" {
							raw = toString(tRes["response"])
						}
						raw = normalizeJSONLikePayload(strings.TrimSpace(raw))
						if raw != "" {
							translationsJSON = raw
						}
					} else {
						parseFailed = true
					}
				}

				results = append(results, sentenceResult{
					sentence:     sent,
					segments:     segments,
					translations: translationsJSON,
					reconOK:      reconOK,
					parseFailed:  parseFailed,
				})
			}

			// Build combined output.
			type segTranslation struct {
				Text    string `json:"text"`
				Pinyin  string `json:"pinyin"`
				English string `json:"english"`
			}

			var allTranslations []segTranslation
			overallReconOK := true
			overallParseFailed := allParseFailed
			for _, sr := range results {
				if !sr.reconOK {
					overallReconOK = false
				}
				if sr.parseFailed {
					overallParseFailed = true
				}
				var trans []segTranslation
				_ = json.Unmarshal([]byte(sr.translations), &trans)
				// Match translations back to CJK segments.
				tIdx := 0
				for _, seg := range sr.segments {
					seg = strings.TrimSpace(seg)
					if seg == "" || shouldSkipSegment(seg) {
						allTranslations = append(allTranslations, segTranslation{Text: seg})
						continue
					}
					if tIdx < len(trans) {
						allTranslations = append(allTranslations, segTranslation{
							Text:    seg,
							Pinyin:  trans[tIdx].Pinyin,
							English: trans[tIdx].English,
						})
						tIdx++
					} else {
						allTranslations = append(allTranslations, segTranslation{Text: seg})
					}
				}
			}

			transJSON, _ := json.Marshal(allTranslations)

			return map[string]interface{}{
				"translations_json": string(transJSON),
				"paragraph":         paragraph,
				"reconstruction_ok": overallReconOK,
				"parse_failed":      overallParseFailed,
				"latency_ms":        float64(time.Since(start).Milliseconds()),
			}, nil
		},
	}
}
```

Note: `shouldSkipSegment` is in the `translation` sub-package. Since `gepa_harness.go` is in `scripts/segmentation`, it can't import internal packages directly. Add a local copy of `shouldSkipSegment` and `isCJKIdeograph` to the harness (they already have local copies of `parseSegments` etc.).

- [ ] **Step 3: Add `shouldSkipSegment` and `isCJKIdeograph` to harness**

```go
func isCJKIdeograph(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2CEAF) ||
		(r >= 0x2CEB0 && r <= 0x2EBEF) ||
		(r >= 0x30000 && r <= 0x323AF)
}

func shouldSkipSegment(segment string) bool {
	if strings.TrimSpace(segment) == "" {
		return true
	}
	for _, r := range segment {
		if isCJKIdeograph(r) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Implement `fullPipelineMetric`**

```go
const defaultTranslateInstruction = "Given an array of Chinese word segments from a sentence, produce the pinyin (with tone marks) and a concise English translation for each segment. Use the sentence and full text for context to select the correct reading and meaning. Return a JSON array of objects with \"pinyin\" and \"english\" fields, in the same order as the input segments."

func fullPipelineMetric(judgeLLM core.LLM) func(expected, actual map[string]interface{}) float64 {
	judgeSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("evaluation_input", core.WithDescription("The segmentation and translation data to evaluate"))},
		},
		[]core.OutputField{
			{Field: core.NewField("score", core.WithDescription("Quality score from 0 to 10"))},
		},
	).WithInstruction("Rate the quality of Chinese word segmentations and translations. Consider: Are word boundaries natural? Is each pinyin correct for this context? Is each English meaning correct for this context? Return JSON with a single \"score\" field (integer 0-10).")

	judge := modules.NewPredict(judgeSig).WithStructuredOutput()
	judge.SetLLM(judgeLLM)

	return func(expected, actual map[string]interface{}) float64 {
		paragraph := strings.TrimSpace(toString(expected["paragraph"]))
		if paragraph == "" {
			paragraph = strings.TrimSpace(toString(actual["paragraph"]))
		}
		if paragraph == "" {
			return 0
		}

		score := 0.0

		// Reconstruction check.
		if !isTruthy(actual["reconstruction_ok"]) {
			score -= 0.45
		}

		// Parse check.
		if isTruthy(actual["parse_failed"]) {
			score -= 0.35
		}

		// Judge score.
		translationsJSON := toString(actual["translations_json"])
		if translationsJSON == "" || translationsJSON == "[]" || translationsJSON == "null" {
			return boundFloat(score, 0, 1)
		}

		evalInput := fmt.Sprintf("Paragraph: %s\n\nSegment translations:\n%s", paragraph, formatTranslationsForJudge(translationsJSON))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := judge.Process(ctx, map[string]any{"evaluation_input": evalInput})
		if err != nil {
			// Judge failure — return reconstruction/parse score only.
			return boundFloat(score+0.5, 0, 1)
		}

		judgeScore := parseJudgeScore(res["score"])
		if judgeScore < 0 {
			judgeScore = parseJudgeScore(res["response"])
		}
		if judgeScore < 0 {
			return boundFloat(score+0.5, 0, 1)
		}

		score += float64(judgeScore) / 10.0

		// Latency penalty.
		latencyMs := toFloat64(actual["latency_ms"])
		if latencyMs > 0 {
			score -= minFloat(0.05, latencyMs/10000.0)
		}

		return boundFloat(score, 0, 1)
	}
}

func formatTranslationsForJudge(translationsJSON string) string {
	type segTrans struct {
		Text    string `json:"text"`
		Pinyin  string `json:"pinyin"`
		English string `json:"english"`
	}
	var translations []segTrans
	if err := json.Unmarshal([]byte(translationsJSON), &translations); err != nil {
		return translationsJSON
	}
	var b strings.Builder
	for i, t := range translations {
		if t.Text == "" || (t.Pinyin == "" && t.English == "") {
			continue
		}
		fmt.Fprintf(&b, "%d. %s — %s — %s\n", i+1, t.Text, t.Pinyin, t.English)
	}
	return b.String()
}

func parseJudgeScore(v any) int {
	if v == nil {
		return -1
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		t = strings.TrimSpace(t)
		t = normalizeJSONLikePayload(t)
		var payload map[string]any
		if err := json.Unmarshal([]byte(t), &payload); err == nil {
			if s, ok := payload["score"]; ok {
				return parseJudgeScore(s)
			}
		}
		// Try direct integer parse.
		var n int
		if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
			return n
		}
	case map[string]any:
		if s, ok := t["score"]; ok {
			return parseJudgeScore(s)
		}
	}
	return -1
}

func boundFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
```

- [ ] **Step 5: Verify compilation**

```bash
cd server && go vet ./scripts/segmentation/...
```

Expected: May have errors in `RunMultiSeedOptimization` and evaluation functions that still reference old types — fixed in next task.

- [ ] **Step 6: Commit**

```bash
git add server/scripts-go/segmentation/gepa_harness.go
git commit -m "feat: implement full pipeline program and judge metric for GEPA

NewFullPipelineProgram runs segment+translate per sentence.
fullPipelineMetric uses judge LLM to score translation quality 0-10."
```

---

## Task 10: Update GEPA Orchestration and CLI

Wire everything together — `RunMultiSeedOptimization`, evaluation, and CLI flags.

**Files:**
- Modify: `server/scripts-go/segmentation/gepa_harness.go`
- Modify: `server/cmd/gepa-segmentation/main.go`

- [ ] **Step 1: Update `CompileGEPASentenceLevel`**

Rename to `CompileFullPipeline` and update to use the new program and metric:

```go
func CompileFullPipeline(
	ctx context.Context,
	workerLLM core.LLM,
	judgeLLM core.LLM,
	corpus []Case,
	baseInstruction string,
	cfg *optimizers.GEPAConfig,
	maxDatasetUnits int,
) (CompileResult, error) {
	dataset, units := BuildGEPAParagraphDataset(corpus, maxDatasetUnits)
	if len(units) == 0 {
		return CompileResult{}, fmt.Errorf("empty GEPA dataset")
	}

	program := NewFullPipelineProgram(workerLLM, baseInstruction, defaultTranslateInstruction)
	metric := fullPipelineMetric(judgeLLM)
	gepa, err := optimizers.NewGEPA(cfg)
	if err != nil {
		return CompileResult{}, fmt.Errorf("new GEPA: %w", err)
	}

	compileCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	start := time.Now()
	optimizedProgram, err := gepa.Compile(compileCtx, program, dataset, metric)
	if err != nil {
		return CompileResult{}, err
	}
	elapsed := time.Since(start)

	state := gepa.GetOptimizationState()
	bestInstruction := extractInstructionFromProgram(optimizedProgram, "segmenter")
	if state != nil && state.BestCandidate != nil && strings.TrimSpace(state.BestCandidate.Instruction) != "" {
		bestInstruction = state.BestCandidate.Instruction
	}
	if strings.TrimSpace(bestInstruction) == "" {
		return CompileResult{}, fmt.Errorf("compiled instruction is empty")
	}

	return CompileResult{
		CompileElapsed:   elapsed,
		BestInstruction:  bestInstruction,
		OptimizedProgram: optimizedProgram,
		State:            state,
		DatasetUnits:     len(units),
	}, nil
}
```

- [ ] **Step 2: Update `EvaluateSentenceLevelProgram`**

Rename to `EvaluateFullPipeline`:

```go
func EvaluateFullPipeline(ctx context.Context, program core.Program, judgeLLM core.LLM, corpus []Case) EvalSummary {
	metric := fullPipelineMetric(judgeLLM)
	summary := EvalSummary{TotalCases: len(corpus)}
	for _, tc := range corpus {
		start := time.Now()
		res, err := program.Execute(ctx, map[string]interface{}{"paragraph": tc.Paragraph})
		latency := time.Since(start)
		if err != nil {
			summary.Errors++
			summary.ReconstructionFail++
			summary.TotalLatency += latency
			continue
		}
		summary.TotalLatency += latency

		if !isTruthy(res["reconstruction_ok"]) {
			summary.ReconstructionFail++
		}

		// Use the metric to get the score.
		expected := map[string]interface{}{"paragraph": tc.Paragraph}
		score := metric(expected, res)
		if score >= 1.0 {
			summary.ExactMatches++
		}
	}
	return summary
}
```

- [ ] **Step 3: Update `RunMultiSeedOptimization` signature and body**

Add `judgeLLM` parameter:

```go
func RunMultiSeedOptimization(
	ctx context.Context,
	workerLLM core.LLM,
	judgeLLM core.LLM,
	modelID string,
	allCases []Case,
	datasetPath string,
	seeds int,
	baseSeed int,
	trainRatio float64,
	maxUnits int,
	cfg *optimizers.GEPAConfig,
) ([]SeedRunResult, CampaignSummary, PromotionDecision, error) {
```

Inside the loop, update compile and eval calls:

```go
comp, err := CompileFullPipeline(ctx, workerLLM, judgeLLM, train, baseInstruction, cfg, len(train))
// ...
baselineProgram := NewFullPipelineProgram(workerLLM, HardenedInstruction, defaultTranslateInstruction)
compiledProgram := NewFullPipelineProgram(workerLLM, comp.BestInstruction, defaultTranslateInstruction)
baselineEval := EvaluateFullPipeline(ctx, baselineProgram, judgeLLM, eval)
compiledEval := EvaluateFullPipeline(ctx, compiledProgram, judgeLLM, eval)
```

- [ ] **Step 4: Update CLI**

In `server/cmd/gepa-segmentation/main.go`:

Add judge model flag and create judge LLM:

```go
judgeModelOverride := flag.String("judge-model", "", "model id for the judge LLM (defaults to worker model)")
// ... existing flags ...

flag.Parse()

_ = godotenv.Load()

cfg, err := config.Load()
if err != nil {
	log.Fatalf("failed to load config: %v", err)
}
if override := strings.TrimSpace(*modelOverride); override != "" {
	cfg.OpenAITranslationModel = override
}

// ... existing LLM setup ...

judgeModelID := cfg.OpenAITranslationModel
if override := strings.TrimSpace(*judgeModelOverride); override != "" {
	judgeModelID = override
}
judgeLLM, err := segmentation.NewSegmentationLLM(cfg, judgeModelID)
if err != nil {
	log.Fatalf("failed to initialize judge llm: %v", err)
}
```

Update the `RunMultiSeedOptimization` call to pass `judgeLLM`:

```go
runs, summary, decision, err := segmentation.RunMultiSeedOptimization(
	context.Background(),
	llm,
	judgeLLM,
	cfg.OpenAITranslationModel,
	corpus,
	*datasetPath,
	*seeds,
	*baseSeed,
	*trainRatio,
	*maxUnits,
	gepaCfg,
)
```

Update the default dataset path:

```go
datasetPath := flag.String("dataset", segmentation.DefaultCSVPath, "CSV dataset path (paragraph-level)")
```

- [ ] **Step 5: Remove old segmentation-only functions**

Remove or mark as deprecated:
- `NewGEPASegmentationProgram` (replaced by `NewFullPipelineProgram`)
- `gepaSentenceMetric` (replaced by `fullPipelineMetric`)
- `boundaryF1FromSegments`, `segmentationBoundaries` (no longer used)
- `CompileGEPASentenceLevel` (replaced by `CompileFullPipeline`)
- `EvaluateSentenceLevelProgram` (replaced by `EvaluateFullPipeline`)

- [ ] **Step 6: Verify full compilation**

```bash
cd server && go build ./...
```

Expected: Compiles cleanly.

- [ ] **Step 7: Run all unit tests**

```bash
cd server && go test ./... -count=1
```

Expected: All tests pass.

- [ ] **Step 8: Format and commit**

```bash
cd server && gofmt -w .
git add -A
git commit -m "feat: wire full-pipeline GEPA evaluation end-to-end

Update CompileFullPipeline, EvaluateFullPipeline, RunMultiSeedOptimization
to use paragraph dataset and judge LLM. Add --judge-model CLI flag.
Remove old segmentation-only metric and evaluation functions."
```

---

## Task 11: Integration Test with Upstream LLM

Update the upstream integration test to verify the batched translation works end-to-end.

**Files:**
- Modify: `server/tests/integration/upstream_llm_test.go`

- [ ] **Step 1: Update the test**

The existing test calls `POST /api/translations/segments/batch`. Update it to match the new handler signature:

```go
func TestUpstreamTranslateBatch(t *testing.T) {
	t.Parallel()
	cfg := loadUpstreamConfig(t)
	router := newRouterWithConfig(cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	res := doJSONRequest(t, router, http.MethodPost, "/api/translations/segments/batch", map[string]any{
		"segments": []string{"人工智能", "改变", "世界"},
		"context":  "人工智能改变世界",
	}, sessionCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
}
```

The handler already uses `derefOr(req.Context, "")` for both `sentence` and `fullText` params, so the request body doesn't change.

- [ ] **Step 2: Commit**

```bash
git add server/tests/integration/upstream_llm_test.go
git commit -m "test: update upstream integration test for new TranslateSegments"
```

---

## Task 12: Final Cleanup and Format

**Files:**
- All modified files

- [ ] **Step 1: Run gofmt**

```bash
cd server && gofmt -w .
```

- [ ] **Step 2: Run full test suite**

```bash
cd server && go test ./... -count=1 -v 2>&1 | tail -30
```

Expected: All tests pass.

- [ ] **Step 3: Run vet**

```bash
cd server && go vet ./...
```

Expected: No issues.

- [ ] **Step 4: Remove `UI_ISSUES.md` if desired**

The untracked `UI_ISSUES.md` was a working document for the UI audit. It can stay or be removed — it's not part of this change.

- [ ] **Step 5: Final commit if any formatting changes**

```bash
cd server && gofmt -w .
git add -A
git diff --cached --stat
# Only commit if there are changes
git commit -m "chore: gofmt cleanup" || true
```
