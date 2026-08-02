package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/UjjwalVandur/TestBud/internal/aigenerator"
	"github.com/UjjwalVandur/TestBud/internal/api"
	"github.com/UjjwalVandur/TestBud/internal/config"
	"github.com/UjjwalVandur/TestBud/internal/database"
	"github.com/UjjwalVandur/TestBud/internal/executor"
	"github.com/UjjwalVandur/TestBud/internal/generator"
	"github.com/UjjwalVandur/TestBud/internal/parser"
	"github.com/UjjwalVandur/TestBud/internal/repository"
	"github.com/UjjwalVandur/TestBud/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load(".")
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}

	schemaRepo := repository.NewGormSchemaRepository(db)
	userRepo := repository.NewGormUserRepository(db)
	execRepo := repository.NewGormExecutionRepository(db)
	coverageRepo := repository.NewGormCoverageRepository(db)

	ruleGen := generator.NewGenerator()
	var testGen service.TestCaseGenerator = ruleGen

	if cfg.BedrockRegion != "" && cfg.BedrockModelID != "" {
		aiGen, err := aigenerator.New(cfg.BedrockRegion, cfg.BedrockModelID, aigenerator.WithLogger(logger))
		if err != nil {
			logger.Warn("ai generator init failed, using rule-based only", "error", err)
		} else {
			testGen = generator.NewCompositeGenerator(ruleGen, aiGen, logger)
			logger.Info("ai-powered test case generation enabled (AWS Bedrock / Gemma 4)", 
				"region", cfg.BedrockRegion,
				"model_id", cfg.BedrockModelID,
			)
		}
	}

	schemaService := service.NewSchemaService(parser.NewParser(), schemaRepo, testGen)
	executionService := service.NewExecutionService(schemaRepo, execRepo, executor.NewExecutor(), logger)
	coverageService := service.NewCoverageService(schemaRepo, execRepo, coverageRepo, logger)
	regressionService := service.NewRegressionService(schemaRepo, logger)

	router := api.NewRouter(api.RouterDependencies{
		Logger:            logger,
		SchemaService:     schemaService,
		ExecutionService:  executionService,
		CoverageService:   coverageService,
		RegressionService: regressionService,
		UserLookup:        userRepo,
		CORSOrigins:       cfg.CORSOrigins,
		ClerkSecretKey:    cfg.ClerkSecretKey,
	})

	// 90-day execution retention — runs daily.
	retentionStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cutoff := time.Now().UTC().AddDate(0, 0, -90)
				deleted, err := execRepo.DeleteOldExecutions(context.Background(), cutoff)
				if err != nil {
					logger.Error("execution retention cleanup failed", "error", err)
					continue
				}
				if deleted > 0 {
					logger.Info("execution retention cleanup completed", "deleted", deleted)
				}
			case <-retentionStop:
				return
			}
		}
	}()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("api server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-shutdownCtx.Done()

	// Graceful shutdown: stop retention goroutine first, then the HTTP server.
	close(retentionStop)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
	}
}
