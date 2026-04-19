"""
Segmentation instruction optimizer using dspy.GEPA (Python dspy).

Usage (from project root):
    cd scripts-py && uv sync
    uv run python gepa_segmentation.py --dataset data/jepa/datasets/paragraphs.csv

Or with a dedicated reflection model:
    uv run python gepa_segmentation.py --model gpt-4o --reflection-model gpt-4o

The script reads paragraphs from a CSV (columns: id, paragraph), runs GEPA
instruction optimization across multiple seeds, and writes artifacts under
data/jepa/ — with compiled_instruction.txt at the artifact root and run
metadata under data/jepa/runs/.
"""

import argparse
import csv
import json
import logging
import os
import random
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path

import dspy
from dotenv import load_dotenv

logger = logging.getLogger(__name__)

REPO_ROOT = Path(__file__).resolve().parent.parent

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

DEFAULT_DATA_DIR = Path("data") / "jepa"
DEFAULT_RUNS_DIR = DEFAULT_DATA_DIR / "runs"
DEFAULT_CSV_PATH = str(DEFAULT_DATA_DIR / "datasets" / "paragraphs.csv")
DEFAULT_ARTIFACTS_DIR = str(DEFAULT_DATA_DIR)
DEFAULT_OPENROUTER_BASE_URL = "https://openrouter.ai/api/v1"

HARDENED_INSTRUCTION = (
    "Segment the Chinese input into an ordered JSON array of contiguous chunks that "
    "exactly reconstruct the original text when concatenated. Preserve every character "
    "in order, including Chinese/ASCII punctuation, symbols, and line breaks. Do not "
    "drop, normalize, paraphrase, or insert characters. Keep common multi-character "
    "words together when appropriate (for example, 人工智能, 图书馆, 看书, 为时未晚). "
    "Return only the segments array."
)

# One preference variant per seed, cycling if seeds > 3.
SEED_PREFERENCES = [
    "Prefer lexicalized multi-character words and stable named entities when boundaries are ambiguous.",
    "Prefer semantically coherent compounds while preserving exact punctuation attachment.",
    "Prefer natural spoken-word grouping for particles and function words without breaking reconstruction.",
]

# ---------------------------------------------------------------------------
# Data types
# ---------------------------------------------------------------------------


@dataclass
class Case:
    name: str
    paragraph: str


@dataclass
class EvalSummary:
    exact_matches: int = 0
    total_cases: int = 0
    reconstruction_fail: int = 0
    errors: int = 0

    def accuracy(self) -> float:
        return self.exact_matches / self.total_cases if self.total_cases else 0.0


@dataclass
class SeedRunResult:
    seed: int
    train_size: int
    eval_size: int
    base_instruction: str
    best_instruction: str
    baseline_accuracy: float
    compiled_accuracy: float
    accuracy_delta: float
    recon_delta: int
    errors_delta: int
    promotable: bool
    reject_reasons: list[str]


# ---------------------------------------------------------------------------
# CSV loading
# ---------------------------------------------------------------------------


def load_cases_from_csv(path: str) -> list[Case]:
    cases = []
    with open(path, encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f)
        for row in reader:
            paragraph = (row.get("paragraph") or "").strip()
            if paragraph:
                cases.append(Case(name=(row.get("id") or "").strip(), paragraph=paragraph))
    if not cases:
        raise ValueError(f"No valid rows found in {path!r}")
    return cases


def resolve_repo_path(raw_path: str | Path) -> Path:
    path = Path(raw_path)
    if path.is_absolute():
        return path
    return REPO_ROOT / path


def build_artifact_paths(artifacts_dir: str | Path) -> dict[str, Path]:
    root_dir = Path(artifacts_dir)
    runs_dir = root_dir / "runs"
    return {
        "root_dir": root_dir,
        "runs_dir": runs_dir,
        "compiled_instruction": root_dir / "compiled_instruction.txt",
        "multi_seed_runs": runs_dir / "multi_seed_runs.json",
        "compile_metadata": runs_dir / "compile_metadata.json",
    }


# ---------------------------------------------------------------------------
# Model/env helpers
# ---------------------------------------------------------------------------


def normalize_model_id(model: str, base_url: str) -> str:
    """
    Normalize model ids for LiteLLM/DSPy.

    With an OpenRouter-compatible base URL, LiteLLM rejects OpenRouter model ids like
    `google/gemini-2.5-flash-lite` unless they are routed via the `openrouter/` provider.
    """
    model = model.strip()
    if not model:
        return model

    if "openrouter.ai" in base_url.lower() and model.startswith("google/"):
        return f"openrouter/{model}"

    return model


# ---------------------------------------------------------------------------
# Instruction builder (mirrors Go BuildConstrainedInstruction)
# ---------------------------------------------------------------------------


def build_constrained_instruction(preference: str = "") -> str:
    preference = (
        preference.strip()
        or "Prefer common lexicalized multi-character words while preserving exact text reconstruction."
    )
    return (
        "You are an expert Chinese segmenter. "
        "Non-negotiable constraints: "
        "1) Concatenated output segments must exactly reconstruct original input text. "
        "2) Preserve all punctuation, symbols, ASCII, and whitespace in order. "
        "3) Never insert, delete, normalize, paraphrase, or translate characters. "
        f"Segmentation preference: {preference} "
        "Return only the segments array."
    )


# ---------------------------------------------------------------------------
# dspy program
# ---------------------------------------------------------------------------


class SegmentChinese(dspy.Signature):
    """Segment Chinese text into meaningful chunks that exactly reconstruct the original."""

    text: str = dspy.InputField(desc="Chinese sentence or paragraph to segment")
    segments: list[str] = dspy.OutputField(
        desc="Ordered array of segments that concatenate exactly to the original text, preserving every character"
    )


class Segmenter(dspy.Module):
    def __init__(self) -> None:
        self.predict = dspy.Predict(SegmentChinese)

    def forward(self, text: str) -> dspy.Prediction:
        return self.predict(text=text)


# ---------------------------------------------------------------------------
# Metric (with GEPA textual feedback)
# ---------------------------------------------------------------------------


def segmentation_metric(
    gold: dspy.Example,
    pred: dspy.Prediction,
    trace=None,
    pred_name=None,
    pred_trace=None,
) -> dspy.Prediction:
    """
    Reconstruction accuracy metric with textual feedback for GEPA's reflective evolution.

    GEPA in DSPy 3.x expects metrics with signature:
    (gold, pred, trace, pred_name, pred_trace).

    It also expects either a float score or a dspy.Prediction(score=..., feedback=...).
    Returning a plain dict causes GEPA internals to treat the entire dict as the score.
    """
    text: str = gold.text
    segments = getattr(pred, "segments", None) or []

    if isinstance(segments, str):
        try:
            segments = json.loads(segments)
        except Exception:
            return dspy.Prediction(
                score=0.0,
                feedback=f"Could not parse segments output: {segments!r}. Return a JSON array of strings.",
            )

    if not isinstance(segments, list) or not segments:
        return dspy.Prediction(
            score=0.0,
            feedback="No segments returned. The output must be a non-empty JSON array of strings.",
        )

    reconstructed = "".join(str(s) for s in segments)

    if reconstructed == text:
        return dspy.Prediction(score=1.0, feedback="Perfect reconstruction.")

    # Partial credit based on character overlap, capped at 0.9 for imperfect output.
    overlap = sum(a == b for a, b in zip(reconstructed, text))
    score = min(overlap / max(len(text), 1), 0.9)

    if len(reconstructed) < len(text):
        missing = len(text) - len(reconstructed)
        feedback = (
            f"Reconstruction too short by {missing} character(s). "
            f"Input: {text!r}. Got: {reconstructed!r}. "
            "Segments are missing characters — never drop any part of the input."
        )
    elif len(reconstructed) > len(text):
        extra = len(reconstructed) - len(text)
        feedback = (
            f"Reconstruction too long by {extra} character(s). "
            f"Input: {text!r}. Got: {reconstructed!r}. "
            "Segments contain extra characters — never insert anything not in the input."
        )
    else:
        diffs = [
            (i, text[i], reconstructed[i])
            for i in range(len(text))
            if text[i] != reconstructed[i]
        ]
        feedback = (
            f"Reconstruction length correct but {len(diffs)} character(s) differ. "
            f"First mismatches (pos, expected, got): {diffs[:3]}. "
            "Do not normalize, substitute, or paraphrase any character."
        )

    return dspy.Prediction(score=score, feedback=feedback)


# ---------------------------------------------------------------------------
# Dataset splitting (mirrors Go SplitCasesDeterministic)
# ---------------------------------------------------------------------------


def split_cases(
    cases: list[Case], train_ratio: float, seed: int, max_units: int
) -> tuple[list[Case], list[Case]]:
    rng = random.Random(seed)
    filtered = [c for c in cases if c.paragraph.strip()]
    rng.shuffle(filtered)
    if max_units > 0:
        filtered = filtered[:max_units]
    if len(filtered) < 2:
        return filtered, []
    split = max(1, int(len(filtered) * train_ratio))
    if split >= len(filtered):
        split = len(filtered) - 1
    return filtered[:split], filtered[split:]


def to_examples(cases: list[Case]) -> list[dspy.Example]:
    return [dspy.Example(text=c.paragraph).with_inputs("text") for c in cases]


# ---------------------------------------------------------------------------
# Evaluation (without optimizer)
# ---------------------------------------------------------------------------


def evaluate_program(program: Segmenter, cases: list[Case]) -> EvalSummary:
    summary = EvalSummary(total_cases=len(cases))
    for case in cases:
        try:
            pred = program(text=case.paragraph)
            result = segmentation_metric(dspy.Example(text=case.paragraph), pred)
            score = float(getattr(result, "score", result))
            if score >= 1.0:
                summary.exact_matches += 1
            else:
                summary.reconstruction_fail += 1
        except Exception as e:
            logger.warning("Eval error for case %r: %s", case.name, e)
            summary.errors += 1
            summary.reconstruction_fail += 1
    return summary


# ---------------------------------------------------------------------------
# Promotion gate (mirrors Go EvaluatePromotionGate)
# ---------------------------------------------------------------------------


def evaluate_promotion_gate(
    baseline: EvalSummary, compiled: EvalSummary
) -> tuple[bool, list[str]]:
    reasons = []
    if compiled.accuracy() - baseline.accuracy() <= 0:
        reasons.append("accuracy_delta_not_positive")
    if compiled.reconstruction_fail > baseline.reconstruction_fail:
        reasons.append("reconstruction_failures_increased")
    if compiled.errors > baseline.errors:
        reasons.append("errors_increased")
    return len(reasons) == 0, reasons


# ---------------------------------------------------------------------------
# Single-seed GEPA compilation
# ---------------------------------------------------------------------------


def compile_segmenter(
    base_instruction: str,
    trainset: list[dspy.Example],
    valset: list[dspy.Example],
    reflection_lm: dspy.LM,
    seed: int,
    auto: str = "light",
) -> tuple[Segmenter, str]:
    """Run GEPA for one seed. Returns (optimized_program, best_instruction)."""
    program = Segmenter()
    program.predict.signature = program.predict.signature.with_instructions(base_instruction)

    gepa = dspy.GEPA(
        metric=segmentation_metric,
        reflection_lm=reflection_lm,
        auto=auto,
        num_threads=4,
        seed=seed,
        track_stats=True,
    )
    optimized: Segmenter = gepa.compile(program, trainset=trainset, valset=valset)

    # Primary: instruction on the optimized predictor's signature.
    instruction: str = optimized.predict.signature.instructions

    # Prefer the tracked best candidate if available (more reliable across runs).
    if hasattr(optimized, "detailed_results") and optimized.detailed_results:
        best = getattr(optimized.detailed_results, "best_candidate", None)
        if isinstance(best, dict):
            for _name, inst in best.items():
                if inst and str(inst).strip():
                    instruction = str(inst).strip()
                    break

    if not instruction or not instruction.strip():
        logger.warning("Seed %d: compiled instruction was empty, using base instruction", seed)
        instruction = base_instruction

    return optimized, instruction


# ---------------------------------------------------------------------------
# Multi-seed orchestration (mirrors Go RunMultiSeedOptimization)
# ---------------------------------------------------------------------------


def run_multi_seed_optimization(
    cases: list[Case],
    reflection_lm: dspy.LM,
    seeds: int = 3,
    base_seed: int = 101,
    max_units: int = 20,
    train_ratio: float = 0.7,
    auto: str = "light",
) -> tuple[list[SeedRunResult], str]:
    runs: list[SeedRunResult] = []
    best_instruction = HARDENED_INSTRUCTION
    best_delta = float("-inf")

    for i in range(seeds):
        seed = base_seed + i
        preference = SEED_PREFERENCES[i % len(SEED_PREFERENCES)]
        base_instruction = build_constrained_instruction(preference)

        train_cases, eval_cases = split_cases(cases, train_ratio, seed, max_units)
        if not train_cases or not eval_cases:
            logger.warning("Seed %d: empty train/eval split, skipping", seed)
            continue

        logger.info(
            "Seed %d/%d (seed=%d): train=%d eval=%d instruction_prefix=%r",
            i + 1, seeds, seed, len(train_cases), len(eval_cases),
            base_instruction[:60],
        )

        trainset = to_examples(train_cases)
        valset = to_examples(eval_cases)

        try:
            optimized, instruction = compile_segmenter(
                base_instruction, trainset, valset, reflection_lm, seed=seed, auto=auto
            )
        except Exception as e:
            logger.error("Seed %d compilation failed: %s", seed, e)
            continue

        # Evaluate: hardened baseline vs compiled instruction on the eval set.
        baseline_prog = Segmenter()
        baseline_prog.predict.signature = baseline_prog.predict.signature.with_instructions(
            HARDENED_INSTRUCTION
        )
        baseline_eval = evaluate_program(baseline_prog, eval_cases)
        compiled_eval = evaluate_program(optimized, eval_cases)

        promotable, reasons = evaluate_promotion_gate(baseline_eval, compiled_eval)
        delta = compiled_eval.accuracy() - baseline_eval.accuracy()

        logger.info(
            "Seed %d: baseline_acc=%.3f compiled_acc=%.3f delta=%.3f promotable=%s reasons=%s",
            seed, baseline_eval.accuracy(), compiled_eval.accuracy(), delta, promotable, reasons,
        )

        run = SeedRunResult(
            seed=seed,
            train_size=len(train_cases),
            eval_size=len(eval_cases),
            base_instruction=base_instruction,
            best_instruction=instruction,
            baseline_accuracy=baseline_eval.accuracy(),
            compiled_accuracy=compiled_eval.accuracy(),
            accuracy_delta=delta,
            recon_delta=compiled_eval.reconstruction_fail - baseline_eval.reconstruction_fail,
            errors_delta=compiled_eval.errors - baseline_eval.errors,
            promotable=promotable,
            reject_reasons=reasons,
        )
        runs.append(run)

        if promotable and delta > best_delta:
            best_delta = delta
            best_instruction = instruction

    if not any(r.promotable for r in runs):
        logger.warning(
            "No promotable seed found — keeping hardened instruction as fallback. "
            "Consider increasing --max-units, --seeds, or using a stronger --reflection-model."
        )

    return runs, best_instruction


# ---------------------------------------------------------------------------
# Artifact writing (mirrors Go WriteOptimizationCampaignArtifacts)
# ---------------------------------------------------------------------------


def write_artifacts(
    artifacts_dir: str,
    instruction: str,
    runs: list[SeedRunResult],
    model_id: str,
    dataset_path: str,
) -> None:
    paths = build_artifact_paths(artifacts_dir)
    paths["root_dir"].mkdir(parents=True, exist_ok=True)
    paths["runs_dir"].mkdir(parents=True, exist_ok=True)

    # compiled_instruction.txt — canonical repo-level artifact loaded by the Go server.
    paths["compiled_instruction"].write_text(
        instruction.strip() + "\n", encoding="utf-8"
    )

    # multi_seed_runs.json — per-seed results.
    paths["multi_seed_runs"].write_text(
        json.dumps([asdict(r) for r in runs], indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )

    # compile_metadata.json — campaign summary.
    deltas = [r.accuracy_delta for r in runs]
    promotable_count = sum(1 for r in runs if r.promotable)
    metadata = {
        "model_id": model_id,
        "dataset_path": str(dataset_path),
        "generated_at_utc": datetime.now(timezone.utc).isoformat(),
        "optimizer": "dspy.GEPA",
        "seeds": len(runs),
        "promotable_count": promotable_count,
        "accuracy_delta_mean": sum(deltas) / len(deltas) if deltas else 0.0,
        "best_accuracy_delta": max(deltas) if deltas else 0.0,
        "worst_accuracy_delta": min(deltas) if deltas else 0.0,
    }
    paths["compile_metadata"].write_text(
        json.dumps(metadata, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )

    logger.info(
        "Artifacts written to %s  (promotable: %d/%d)", artifacts_dir, promotable_count, len(runs)
    )


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def main() -> None:
    # Load .env from the current working directory, then fall back to server/.env.
    load_dotenv()
    load_dotenv(dotenv_path=REPO_ROOT / "server" / ".env")

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
        datefmt="%Y-%m-%dT%H:%M:%S",
    )

    parser = argparse.ArgumentParser(
        description="GEPA segmentation instruction optimizer",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument("--dataset", default=DEFAULT_CSV_PATH, help="CSV dataset path")
    parser.add_argument("--artifacts-dir", default=DEFAULT_ARTIFACTS_DIR, help="Output directory for artifacts")
    parser.add_argument("--model", default="", help="Worker model ID (default: OPENAI_TRANSLATION_MODEL env var)")
    parser.add_argument("--reflection-model", default="", help="Reflection LM model ID (default: same as --model)")
    parser.add_argument("--max-units", type=int, default=20, help="Max paragraph units per seed run")
    parser.add_argument("--train-ratio", type=float, default=0.7, help="Train/eval split ratio")
    parser.add_argument("--seeds", type=int, default=3, help="Number of optimization seeds")
    parser.add_argument("--base-seed", type=int, default=101, help="Starting seed value")
    parser.add_argument(
        "--auto", default="light", choices=["light", "medium", "heavy"],
        help="GEPA budget preset controlling number of metric calls"
    )
    args = parser.parse_args()

    api_key = os.environ.get("OPENAI_API_KEY") or os.environ.get("OPENROUTER_API_KEY")
    if not api_key:
        raise SystemExit("OPENAI_API_KEY (or OPENROUTER_API_KEY) must be set in env or .env")

    base_url: str = (
        os.environ.get("OPENAI_BASE_URL")
        or os.environ.get("OPENROUTER_BASE_URL")
        or DEFAULT_OPENROUTER_BASE_URL
    )
    raw_model: str = (
        args.model
        or os.environ.get("OPENAI_TRANSLATION_MODEL")
        or os.environ.get("OPENROUTER_TRANSLATION_MODEL")
        or os.environ.get("OPENAI_MODEL")
        or os.environ.get("OPENROUTER_MODEL", "")
    )
    if not raw_model:
        raise SystemExit(
            "Model must be set via --model or one of: OPENAI_TRANSLATION_MODEL, "
            "OPENROUTER_TRANSLATION_MODEL, OPENAI_MODEL, OPENROUTER_MODEL"
        )

    raw_reflection_model: str = args.reflection_model or raw_model
    model = normalize_model_id(raw_model, base_url)
    reflection_model = normalize_model_id(raw_reflection_model, base_url)

    if model != raw_model:
        logger.info("Normalized worker model for LiteLLM/OpenRouter: %s -> %s", raw_model, model)
    if reflection_model != raw_reflection_model:
        logger.info(
            "Normalized reflection model for LiteLLM/OpenRouter: %s -> %s",
            raw_reflection_model,
            reflection_model,
        )

    lm = dspy.LM(model=model, api_key=api_key, api_base=base_url, cache=False)
    # Reflection LM runs at temperature=1.0 for diversity in instruction proposals.
    reflection_lm = dspy.LM(
        model=reflection_model, api_key=api_key, api_base=base_url, cache=False, temperature=1.0
    )
    dspy.configure(lm=lm)

    dataset_path = resolve_repo_path(args.dataset)
    artifacts_dir = resolve_repo_path(args.artifacts_dir)

    logger.info("Worker: %s  Reflection: %s  Base URL: %s", model, reflection_model, base_url)
    logger.info("Loading dataset from %s", dataset_path)
    cases = load_cases_from_csv(str(dataset_path))
    logger.info("Loaded %d cases", len(cases))

    runs, best_instruction = run_multi_seed_optimization(
        cases,
        reflection_lm=reflection_lm,
        seeds=args.seeds,
        base_seed=args.base_seed,
        max_units=args.max_units,
        train_ratio=args.train_ratio,
        auto=args.auto,
    )

    write_artifacts(
        artifacts_dir=str(artifacts_dir),
        instruction=best_instruction,
        runs=runs,
        model_id=model,
        dataset_path=args.dataset,
    )

    promotable = sum(1 for r in runs if r.promotable)
    logger.info(
        "Campaign complete: seeds=%d promotable=%d/%d",
        len(runs), promotable, len(runs),
    )


if __name__ == "__main__":
    main()
