package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/UjjwalVandur/TestBud/internal/api/handlers"
	"github.com/UjjwalVandur/TestBud/internal/service"
)

type RouterDependencies struct {
	Logger        *logrus.Logger
	SchemaService *service.SchemaService
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(deps.Logger))

	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.Check)

	schemaHandler := handlers.NewSchemaHandler(deps.SchemaService)
	api := router.Group("/api")
	api.POST("/schemas", schemaHandler.Upload)

	return router
}

func requestLogger(logger *logrus.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = logrus.New()
	}

	return func(c *gin.Context) {
		c.Next()
		logger.WithFields(logrus.Fields{
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
			"status": c.Writer.Status(),
		}).Info("request completed")
	}
}
