package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"ImageServer/config"
	"ImageServer/models"
	"ImageServer/utils"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	config *config.Config
}

func NewAPIHandler(cfg *config.Config) *APIHandler {
	return &APIHandler{config: cfg}
}

// ListDirectory handles GET /api/v1/files/*path?list=true
func (h *APIHandler) ListDirectory(c *gin.Context) {
	dirPath := c.Param("path")
	if dirPath == "" {
		dirPath = "/"
	}

	fullPath := filepath.Join(h.config.Path, dirPath)

	files, err := os.ReadDir(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Directory not found"})
		return
	}

	var allFiles []models.FileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}
		if !utils.ContainsDotFile(info.Name()) {
			allFiles = append(allFiles, models.FileInfo{
				Name:    info.Name(),
				Path:    filepath.Join(dirPath, info.Name()),
				Size:    info.Size(),
				ModTime: info.ModTime(),
				IsDir:   info.IsDir(),
			})
		}
	}

	// Get page size from query parameter
	pageSize := 10 // Default page size
	if size := c.Query("size"); size != "" {
		if s, err := strconv.Atoi(size); err == nil && s > 0 {
			pageSize = s
		}
	}

	// Apply pagination
	page := 0
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			page = p
		}
	}

	start := page * pageSize
	if start >= len(allFiles) {
		c.JSON(http.StatusOK, []models.FileInfo{})
		return
	}

	end := start + pageSize
	if end > len(allFiles) {
		end = len(allFiles)
	}

	c.JSON(http.StatusOK, allFiles[start:end])
}

// CreateDirectory handles POST /api/v1/directories/*path
func (h *APIHandler) CreateDirectory(c *gin.Context) {
	dirPath := c.Param("path")
	fullPath := filepath.Join(h.config.Path, dirPath)

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Directory created successfully"})
}

// UploadImage handles POST /api/v1/images
func (h *APIHandler) UploadImage(c *gin.Context) {
	folder := c.PostForm("folder")
	id := c.PostForm("id")

	if folder == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid folder"})
		return
	}

	folderPath := filepath.Join(h.config.Path, folder)
	err := os.MkdirAll(folderPath, 0755)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating folder: " + err.Error()})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error retrieving file: " + err.Error()})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error opening file"})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading uploaded file"})
		return
	}

	buffer := bytes.Clone(fileBytes[0:512])
	contentType := http.DetectContentType(buffer)
	format := strings.Split(contentType, "/")[1]

	if format != "" && !models.SupportedTypes.Has(format) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format"})
		return
	}

	var finalBytes []byte

	if format == "png" {
		finalBytes = fileBytes
	} else {
		// Convert to PNG
		img, _, err2 := image.Decode(bytes.NewReader(fileBytes))
		if err2 != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding image"})
			return
		}
		var buf bytes.Buffer
		if err = png.Encode(&buf, img); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error encoding PNG"})
			return
		}
		finalBytes = buf.Bytes()
	}

	filePath := filepath.Join(folderPath, id)
	outputFile, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating file: " + err.Error()})
		return
	}
	defer outputFile.Close()

	if _, err = outputFile.Write(finalBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving file"})
		return
	}
	baseURL, err := url.Parse(h.config.Domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid domain configuration"})
		return
	}

	baseURL.Path = path.Join(baseURL.Path, folder, id+"."+format)
	c.JSON(http.StatusCreated, gin.H{"url": baseURL.String()})
}

// DeleteFile handles DELETE /api/v1/files/*path
func (h *APIHandler) DeleteFile(c *gin.Context) {
	filePath := c.Param("path")
	fullPath := filepath.Join(h.config.Path, filePath)

	// Get file info to check if it's a directory
	info, err := os.Stat(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Use RemoveAll for directories and Remove for files
	if info.IsDir() {
		if err := os.RemoveAll(fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting directory: " + err.Error()})
			return
		}
	} else {
		if err := os.Remove(fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting file: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully deleted: %s", filePath)})
}
