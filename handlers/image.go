package handlers

import (
	"image/jpeg"
	"image/png"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"ImageServer/config"
	"ImageServer/models"
	"ImageServer/utils"

	"github.com/gin-gonic/gin"
)

type ImageHandler struct {
	config *config.Config
}

func NewImageHandler(cfg *config.Config) *ImageHandler {
	return &ImageHandler{config: cfg}
}

// ServeImage handles image serving at root level (e.g., /path/to/image.png)
func (h *ImageHandler) ServeImage(c *gin.Context) {
	imagePath := c.Param("filepath")

	// Security: Clean the path and prevent directory traversal attacks
	cleanPath := filepath.Clean(imagePath)
	
	// Remove leading slash if present
	if len(cleanPath) > 0 && cleanPath[0] == '/' {
		cleanPath = cleanPath[1:]
	}
	
	// Prevent directory traversal by checking for ".." components
	if filepath.IsAbs(cleanPath) || containsPathTraversal(cleanPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path"})
		return
	}

	// Get absolute path of the configured directory
	baseDir, err := filepath.Abs(h.config.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server configuration error"})
		return
	}

	// Join the cleaned path with the base directory
	filePath := filepath.Join(baseDir, cleanPath)
	
	// Get absolute path of the requested file
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path"})
		return
	}
	
	// Ensure the resolved path is still within the base directory
	if !isWithinDirectory(absFilePath, baseDir) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}


	variant := c.Query("variant")

	// Set caching headers
	c.Header("Cache-Control", "public, max-age=31536000")

	format := path.Ext(filePath)[1:]
	// Get path without extension

	filePathNoExt := filePath[:len(filePath)-len(filepath.Ext(filePath))]

	if format != "" && !models.SupportedTypes.Has(format) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format: " + format})
		return
	}


	// Serve the file directly if no conversion is needed
	if (format == "" || format == "png") && variant == "" {
		c.File(filePathNoExt)
		return
	}

	img, err := utils.ReadImage(filePathNoExt, variant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading image"})
		return
	}

	c.Status(http.StatusOK)
	
	// Encode and send the image in the requested format
	switch format {
	case "jpg", "jpeg":
		c.Header("Content-Type", "image/jpeg")
		if err := jpeg.Encode(c.Writer, img, &jpeg.Options{Quality: 100}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error encoding JPEG"})
		}
	case "png":
		c.Header("Content-Type", "image/png")
		if err := png.Encode(c.Writer, img); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error encoding PNG"})
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format: " + format})
	}
}

// containsPathTraversal checks if the path contains directory traversal sequences
func containsPathTraversal(path string) bool {
	// Check for various forms of path traversal
	return filepath.Clean(path) != path || 
		   filepath.IsAbs(path) ||
		   filepath.VolumeName(path) != "" ||
		   containsTraversalSequences(path)
}

// containsTraversalSequences checks for explicit traversal sequences
func containsTraversalSequences(path string) bool {
	// Normalize path separators to forward slashes
	normalizedPath := filepath.ToSlash(path)
	
	// Split by forward slashes to get path components
	parts := strings.Split(normalizedPath, "/")
	
	// Check each component for traversal sequences
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	
	return false
}

// isWithinDirectory checks if the target path is within the base directory
func isWithinDirectory(targetPath, baseDir string) bool {
	// Convert both paths to absolute paths with consistent separators
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	
	// Ensure both paths end with separator for proper comparison
	if !filepath.IsAbs(targetAbs) || !filepath.IsAbs(baseAbs) {
		return false
	}
	
	// Check if target path starts with base directory path
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	
	// If the relative path starts with "..", it's outside the base directory
	return !filepath.IsAbs(rel) && !containsTraversalSequences(rel)
}
