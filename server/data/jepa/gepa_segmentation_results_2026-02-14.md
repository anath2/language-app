# GEPA Segmentation Results (2026-02-14)

## Setup
- model: google/gemini-2.5-flash-lite
- optimizer: GEPA
- objective: sentence-level segmentation prompt optimization
- dataset source: data/jepa/sentences_20.csv
- dataset size (sentence units): 14

## Quick-Budget Config
- population_size: 12
- max_generations: 6
- evaluation_batch_size: 4
- concurrency_level: 1
- reflection_frequency: 2
- stagnation_limit: 3
- convergence_threshold: 0.0030

## Compile Artifacts
- elapsed: 5m59.691924209s
- best_fitness: 0.9000
- generations_executed: 1

### Best Compiled Instruction
You are an expert Chinese segmenter. Non-negotiable constraints: 1) Concatenated output segments must exactly reconstruct original input text. 2) Preserve all punctuation, symbols, ASCII, and whitespace in order. 3) Never insert, delete, normalize, paraphrase, or translate characters. Segmentation preference: Prefer lexicalized multi-character words and stable named entities when boundaries are ambiguous. Return only the segments array.

## Post-Compile Comparison
- baseline_accuracy: 0.67 (4/6)
- compiled_accuracy: 0.83 (5/6)
- accuracy_delta: 0.17
- baseline_reconstruction_failures: 0
- compiled_reconstruction_failures: 0
- baseline_errors: 0
- compiled_errors: 0
- baseline_avg_latency: 2.228002437s
- compiled_avg_latency: 741.839229ms
- latency_delta: -1.486163208s
