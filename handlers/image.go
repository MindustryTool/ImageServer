package handlers

import (
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"

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
	format := c.Query("format")
	imagePath := c.Param("filepath")

	if format != "" && !models.SupportedTypes.Has(format) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format"})
		return
	}

	filePath := filepath.Join(h.config.Path, imagePath)
	file, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	defer file.Close()

	stats, err := file.Stat()
	if err != nil || stats.IsDir() || utils.ContainsDotFile(file.Name()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	extension := filepath.Ext(file.Name())[1:]

	// Set caching headers
	c.Header("Cache-Control", "public, max-age=31536000")

	// Serve the file directly if no conversion is needed
	if format == "" || format == extension {
		c.File(filePath)
		return
	}

	// Set MIME type based on requested format
	switch format {
	case "png":
		c.Header("Content-Type", "image/png")
	case "jpg", "jpeg":
		c.Header("Content-Type", "image/jpeg")
	case "webp":
		c.Header("Content-Type", "image/webp")
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format"})
		return
	}

	img, err := utils.ReadImage(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading image"})
		return
	}

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format"})
	}
}