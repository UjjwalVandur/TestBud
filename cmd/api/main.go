package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

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
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	cfg, err := config.Load(".")
	if err != nil {
		logger.WithError(err).Fatal("load config")
	}

	db, err := database.Connect(cfg)
	if err != nil {
		logger.WithError(err).Fatal("connect database")
	}

	schemaRepo := repository.NewGormSchemaRepository(db)
	userRepo := repository.NewGormUserRepository(db)
	execRepo := repository.NewGormExecutionRepository(db)
	coverageRepo := repository.NewGormCoverageRepository(db)

	schemaService := service.NewSchemaService(parser.NewParser(), schemaRepo, generator.NewGenerator())
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
	})

	// 90-day execution retention cron — runs daily at 2:00 AM.
	retentionCron := cron.New()
	_, err = retentionCron.AddFunc("0 2 * * *", func() {
		cutoff := time.Now().UTC().AddDate(0, 0, -90)
		deleted, err := execRepo.DeleteOldExecutions(context.Background(), cutoff)
		if err != nil {
			logger.WithError(err).Error("execution retention cleanup failed")
			return
		}
		if deleted > 0 {
			logger.WithField("deleted", deleted).Info("execution retention cleanup completed")
		}
	})
	if err != nil {
		logger.WithError(err).Fatal("schedule retention cron")
	}
	retentionCron.Start()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.WithField("addr", server.Addr).Info("api server listening")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WithError(err).Fatal("api server failed")
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-shutdownCtx.Done()

	// Graceful shutdown: stop cron first, then the HTTP server.
	retentionCron.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("api server shutdown failed")
	}
}
