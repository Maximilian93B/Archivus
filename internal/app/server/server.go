package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/pkg/logger"
)

type Server struct {
	config *config.Config
	logger *logger.Logger
	router *gin.Engine
	server *http.Server
	db     *database.DB
}

// New creates a new server instance
func New(cfg *config.Config, log *logger.Logger) (*Server, error) {
	// Initialize database (optional for development)
	var db *database.DB
	var err error

	if cfg.GetDatabaseURL() != "" {
		db, err = database.New(cfg.GetDatabaseURL())
		if err != nil {
			log.Warn("Failed to initialize database, continuing without database", "error", err)
			db = nil
		}
	}

	// Configure Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware(cfg))
	router.Use(loggingMiddleware(log))

	// Create server
	server := &Server{
		config: cfg,
		logger: log,
		router: router,
		db:     db,
	}

	// Setup routes
	server.setupRoutes()

	return server, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         ":" + s.config.Server.Port,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

	// Close database connection if it exists
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.logger.Error("Error closing database connection", "error", err)
		}
	}

	// Shutdown HTTP server
	return s.server.Shutdown(ctx)
}

// setupRoutes configures all application routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.healthCheck)

	// API v1 group
	v1 := s.router.Group("/api/v1")
	{
		// Public routes
		public := v1.Group("")
		{
			public.GET("/status", s.systemStatus)
		}

		// Protected routes (will be implemented later)
		// protected := v1.Group("")
		// protected.Use(authMiddleware())
		// {
		//     // Document routes, user routes, etc.
		// }
	}
}

// Health check handler
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"timestamp":   time.Now().UTC(),
		"environment": s.config.Environment,
	})
}

// System status handler
func (s *Server) systemStatus(c *gin.Context) {
	// Check database connection
	dbStatus := "not_configured"
	if s.db != nil {
		if err := s.db.Ping(); err != nil {
			dbStatus = "unhealthy"
		} else {
			dbStatus = "healthy"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"database":  dbStatus,
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

// corsMiddleware configures CORS
func corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	corsConfig := cors.Config{
		AllowOrigins:     cfg.Server.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Tenant-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	return cors.New(corsConfig)
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request details
		latency := time.Since(start)
		if raw != "" {
			path = path + "?" + raw
		}

		log.Info("HTTP Request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", latency.String(),
			"client_ip", c.ClientIP(),
		)
	}
}
