package main

import (
	"log"
	"os"
	"path/filepath"

	"ImageServer/config"
	"ImageServer/handlers"
	"ImageServer/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	// Load configuration
	cfg := config.Load()

	// Ensure data directory exists
	dirname, err := filepath.Abs(cfg.Path)
	if err != nil {
		log.Fatalf("Could not get absolute path: %s\n", err)
	}

	dirPath := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Fatalf("Cannot make dir %s: %s\n", cfg.Path, err)
	}

	// Create Gin router
	r := gin.Default()

	// Add middleware
	r.Use(middleware.CORS())

	// Create handlers
	imageHandler := handlers.NewImageHandler(cfg)
	apiHandler := handlers.NewAPIHandler(cfg)

	// REST API routes with /api/v1 prefix
	api := r.Group("/api/v1")
	{
		// Protected routes requiring authentication
		protected := api.Group("/")
		protected.Use(middleware.BasicAuth(cfg.Username, cfg.Password))
		{
			// File operations
			protected.GET("/files/*path", apiHandler.ListDirectory)
			protected.DELETE("/files/*path", apiHandler.DeleteFile)
			
			// Directory operations
			protected.POST("/directories/*path", apiHandler.CreateDirectory)
			
			// Image upload
			protected.POST("/images", apiHandler.UploadImage)
		}
	}

	// Handle all other routes as image serving (fallback for unmatched routes)
	r.NoRoute(func(c *gin.Context) {
		// Only handle GET requests for image serving
		if c.Request.Method == "GET" {
			// Set the filepath parameter for the image handler
			c.Params = append(c.Params, gin.Param{Key: "filepath", Value: c.Request.URL.Path})
			imageHandler.ServeImage(c)
		} else {
			c.JSON(404, gin.H{"error": "Not found"})
		}
	})

	log.Printf("Serving %s on port %s\n", dirname, cfg.Port)

	// Start server
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
