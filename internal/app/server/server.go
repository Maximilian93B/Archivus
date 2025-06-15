package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/app/handlers"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/pkg/logger"
	"github.com/gin-contrib/cors"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	config   *config.Config
	router   *gin.Engine
	server   *http.Server
	handlers *Handlers
	logger   *logger.Logger
}

// Handlers holds all HTTP handlers
type Handlers struct {
	AuthHandler     *handlers.AuthHandler
	DocumentHandler *handlers.DocumentHandler
	// Add other handlers as they're created
}

// NewServer creates a new HTTP server instance
func NewServer(
	cfg *config.Config,
	services *Services,
	logger *logger.Logger,
) *Server {
	// Set Gin mode based on environment
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Create handlers
	handlers := &Handlers{
		AuthHandler:     handlers.NewAuthHandler(services.UserService, services.TenantService, nil),
		DocumentHandler: handlers.NewDocumentHandler(services.DocumentService, services.UserService),
	}

	server := &Server{
		config:   cfg,
		router:   router,
		handlers: handlers,
		logger:   logger,
	}

	server.setupMiddleware()
	server.setupRoutes()

	return server
}

// Services holds all business services
type Services struct {
	UserService      *services.UserService
	TenantService    *services.TenantService
	DocumentService  *services.DocumentService
	WorkflowService  *services.WorkflowService
	AIService        *services.AIService
	AnalyticsService *services.AnalyticsService
}

// setupMiddleware configures all middleware
func (s *Server) setupMiddleware() {
	// Recovery middleware
	s.router.Use(gin.Recovery())

	// Logging middleware
	s.router.Use(s.loggingMiddleware())

	// Security middleware
	s.router.Use(s.securityMiddleware())

	// CORS middleware
	s.router.Use(cors.New(cors.Config{
		AllowOrigins:     s.getAllowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Tenant"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Request size limit middleware
	s.router.Use(s.requestSizeLimitMiddleware())

	// Rate limiting middleware (placeholder)
	s.router.Use(s.rateLimitMiddleware())
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/ready", s.readinessCheck)

	// API version 1
	v1 := s.router.Group("/api/v1")
	{
		// Register handler routes
		s.handlers.AuthHandler.SetupRoutes(v1)
		s.handlers.DocumentHandler.RegisterRoutes(v1)

		// Add other handler routes as they're created
		// s.handlers.WorkflowHandler.RegisterRoutes(v1)
		// s.handlers.AnalyticsHandler.RegisterRoutes(v1)
	}

	// Serve static files (if any)
	s.router.Static("/static", "./web/static")

	// Catch-all route for SPA
	s.router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "route_not_found",
			"message": "The requested route does not exist",
			"path":    c.Request.URL.Path,
		})
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Create HTTP server
	s.server = &http.Server{
		Addr:           ":" + s.config.Server.Port,
		Handler:        s.router,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start server in a goroutine
	go func() {
		s.logger.Info("Starting HTTP server", "port", s.config.Server.Port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Failed to start server", "error", err)
		}
	}()

	return s.waitForShutdown()
}

// StartTLS starts the HTTPS server
func (s *Server) StartTLS(certFile, keyFile string) error {
	// Create HTTPS server
	s.server = &http.Server{
		Addr:           ":" + s.config.Server.Port,
		Handler:        s.router,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start HTTPS server in a goroutine
	go func() {
		s.logger.Info("Starting HTTPS server", "port", s.config.Server.Port)
		if err := s.server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Failed to start HTTPS server", "error", err)
		}
	}()

	return s.waitForShutdown()
}

// waitForShutdown waits for shutdown signal and gracefully shuts down the server
func (s *Server) waitForShutdown() error {
	// Create channel to receive OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	<-quit
	s.logger.Info("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Server forced to shutdown", "error", err)
		return err
	}

	s.logger.Info("Server exited")
	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Middleware functions

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// securityMiddleware adds security headers
func (s *Server) securityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Only add HSTS header for HTTPS
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Content Security Policy (adjust as needed)
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'")

		c.Next()
	}
}

// requestSizeLimitMiddleware limits request body size
func (s *Server) requestSizeLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set different limits based on route
		var maxSize int64 = 1 << 20 // 1MB default

		// Larger limit for file uploads
		if c.Request.URL.Path == "/api/v1/documents/upload" {
			maxSize = s.config.Limits.MaxFileSize // From config
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

// rateLimitMiddleware implements rate limiting (placeholder)
func (s *Server) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement rate limiting using Redis
		// For now, just pass through
		c.Next()
	}
}

// Health check handlers

// healthCheck returns server health status
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"version":     "1.0.0",
		"environment": s.config.Environment,
	})
}

// readinessCheck checks if server is ready to handle requests
func (s *Server) readinessCheck(c *gin.Context) {
	// TODO: Add checks for database, Redis, external services
	checks := map[string]string{
		"database": "ok",
		"redis":    "ok",
		"storage":  "ok",
	}

	allHealthy := true
	for _, status := range checks {
		if status != "ok" {
			allHealthy = false
			break
		}
	}

	statusCode := http.StatusOK
	if !allHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"status": map[string]string{"ready": func() string {
			if allHealthy {
				return "true"
			} else {
				return "false"
			}
		}()},
		"checks":    checks,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Helper functions

// getAllowedOrigins returns allowed CORS origins based on environment
func (s *Server) getAllowedOrigins() []string {
	if s.config.Environment == "production" {
		// In production, only allow specific domains
		return s.config.Server.AllowedOrigins
	}

	// In development, allow common local development origins
	return []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"http://localhost:8080",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:3001",
		"http://127.0.0.1:8080",
	}
}

// GetRouter returns the Gin router (useful for testing)
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// GetServer returns the HTTP server instance
func (s *Server) GetServer() *http.Server {
	return s.server
}
