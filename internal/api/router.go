package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/UjjwalVandur/TestBud/internal/api/handlers"
	"github.com/UjjwalVandur/TestBud/internal/api/middleware"
)

// RouterDependencies uses interfaces so the router is testable without concrete
// service/repository implementations (DEV-1 fix).
type RouterDependencies struct {
	Logger            *slog.Logger
	SchemaService     handlers.SchemaService
	ExecutionService  handlers.ExecutionService
	CoverageService   handlers.CoverageReporter
	RegressionService handlers.RegressionReporter
	UserLookup        middleware.UserLookup
	CORSOrigins       []string
	ClerkSecretKey    string
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(deps.Logger))
	router.Use(middleware.CORS(deps.CORSOrigins))

	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.Check)

	schemaHandler := handlers.NewSchemaHandler(deps.SchemaService)
	executionHandler := handlers.NewExecutionHandler(deps.ExecutionService)
	coverageHandler := handlers.NewCoverageHandler(deps.CoverageService)
	regressionHandler := handlers.NewRegressionHandler(deps.RegressionService)
	api := router.Group("/api")
	api.Use(middleware.AuthMiddleware(deps.UserLookup, deps.ClerkSecretKey))
	api.GET("/schemas", schemaHandler.List)
	api.POST("/schemas", schemaHandler.Upload)
	api.GET("/schemas/:id", schemaHandler.GetByID)
	api.POST("/schemas/:id/executions", executionHandler.Execute)
	api.GET("/schemas/:id/executions", executionHandler.List)
	api.GET("/schemas/:id/coverage", coverageHandler.Get)
	api.GET("/schemas/:id/regression", regressionHandler.Get)
	api.GET("/endpoints/:endpoint_id/testcases", schemaHandler.GetTestCases)

	return router
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		c.Next()
		logger.Info("request completed", 
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
		)
	}
}
