package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/UjjwalVandur/TestBud/internal/api"
	"github.com/UjjwalVandur/TestBud/internal/config"
	"github.com/UjjwalVandur/TestBud/internal/database"
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
	schemaService := service.NewSchemaService(parser.NewParser(), schemaRepo)
	router := api.NewRouter(api.RouterDependencies{
		Logger:        logger,
		SchemaService: schemaService,
		UserLookup:    userRepo,
	})

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("api server shutdown failed")
	}
}
