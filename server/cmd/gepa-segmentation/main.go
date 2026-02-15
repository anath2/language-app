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
	maxUnits := flag.Int("max-units", 20, "max sentence units to use during each seed run")
	trainRatio := flag.Float64("train-ratio", 0.7, "train split ratio (rest used for holdout evaluation)")
	seeds := flag.Int("seeds", 3, "number of optimization seeds")
	baseSeed := flag.Int("base-seed", 101, "starting seed value")
	population := flag.Int("population", 8, "GEPA population size")
	generations := flag.Int("generations", 4, "GEPA max generations")
	evalBatch := flag.Int("eval-batch", 3, "GEPA evaluation batch size")
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

	gepaCfg := segmentation.ModerateFastGEPAConfig()
	gepaCfg.PopulationSize = *population
	gepaCfg.MaxGenerations = *generations
	gepaCfg.EvaluationBatchSize = *evalBatch
	runs, summary, decision, err := segmentation.RunMultiSeedOptimization(
		context.Background(),
		llm,
		cfg.OpenAIModel,
		corpus,
		*datasetPath,
		*seeds,
		*baseSeed,
		*trainRatio,
		*maxUnits,
		gepaCfg,
	)
	if err != nil {
		log.Fatalf("multi-seed optimization failed: %v", err)
	}

	if err := segmentation.WriteOptimizationCampaignArtifacts(
		*artifactsDir,
		cfg.OpenAIModel,
		*datasetPath,
		gepaCfg,
		runs,
		summary,
		decision,
	); err != nil {
		log.Fatalf("failed to write artifacts: %v", err)
	}

	log.Printf(
		"gepa campaign complete model=%s seeds=%d promotable=%d mean_acc_delta=%.3f promoted=%t artifacts_dir=%s",
		cfg.OpenAIModel,
		summary.Seeds,
		summary.PromotableCount,
		summary.AccuracyDeltaMean,
		decision.Promoted,
		*artifactsDir,
	)
}
