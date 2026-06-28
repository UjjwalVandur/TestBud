package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/UjjwalVandur/TestBud/internal/api/handlers"
	"github.com/UjjwalVandur/TestBud/internal/api/middleware"
)

// RouterDependencies uses interfaces so the router is testable without concrete
// service/repository implementations (DEV-1 fix).
type RouterDependencies struct {
	Logger        *logrus.Logger
	SchemaService handlers.SchemaUploader
	UserLookup    middleware.UserLookup
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
	api.Use(middleware.APIKeyAuth(deps.UserLookup))
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
