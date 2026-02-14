package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/scripts/segmentation"
	"github.com/joho/godotenv"
)

func main() {
	datasetPath := flag.String("dataset", segmentation.DefaultCSVPath, "CSV dataset path (sentence-level)")
	artifactsDir := flag.String("artifacts-dir", segmentation.DefaultArtifactsDir, "output directory for GEPA artifacts")
	modelOverride := flag.String("model", "", "override model id (defaults to OPENAI_MODEL)")
	maxUnits := flag.Int("max-units", 20, "max sentence units to use for quick GEPA compile")
	flag.Parse()

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if override := strings.TrimSpace(*modelOverride); override != "" {
		cfg.OpenAIModel = override
	}

	corpus, err := segmentation.LoadCasesFromCSV(*datasetPath)
	if err != nil {
		log.Fatalf("failed to load dataset %q: %v", *datasetPath, err)
	}
	log.Printf("loaded sentence dataset: rows=%d path=%s", len(corpus), *datasetPath)

	llm, err := segmentation.NewSegmentationLLM(cfg, cfg.OpenAIModel)
	if err != nil {
		log.Fatalf("failed to initialize segmentation llm: %v", err)
	}
	core.SetDefaultLLM(llm)
	core.GlobalConfig.TeacherLLM = llm

	gepaCfg := segmentation.QuickBudgetGEPAConfig()
	result, err := segmentation.CompileGEPASentenceLevel(
		context.Background(),
		llm,
		corpus,
		segmentation.HardenedInstruction,
		gepaCfg,
		*maxUnits,
	)
	if err != nil {
		log.Fatalf("gepa compile failed: %v", err)
	}

	baselineProgram := segmentation.NewGEPASegmentationProgram(llm, segmentation.HardenedInstruction)
	compiledProgram := segmentation.NewGEPASegmentationProgram(llm, result.BestInstruction)
	baselineEval := segmentation.EvaluateSentenceLevelProgram(context.Background(), baselineProgram, corpus)
	compiledEval := segmentation.EvaluateSentenceLevelProgram(context.Background(), compiledProgram, corpus)

	if err := segmentation.WriteGEPAArtifacts(
		*artifactsDir,
		cfg.OpenAIModel,
		*datasetPath,
		gepaCfg,
		result,
		baselineEval,
		compiledEval,
	); err != nil {
		log.Fatalf("failed to write artifacts: %v", err)
	}

	log.Printf(
		"gepa complete model=%s dataset_units=%d compile_elapsed=%s baseline_acc=%.2f compiled_acc=%.2f acc_delta=%.2f artifacts_dir=%s",
		cfg.OpenAIModel,
		result.DatasetUnits,
		result.CompileElapsed,
		segmentation.AccuracyOf(baselineEval),
		segmentation.AccuracyOf(compiledEval),
		segmentation.AccuracyOf(compiledEval)-segmentation.AccuracyOf(baselineEval),
		*artifactsDir,
	)
}
