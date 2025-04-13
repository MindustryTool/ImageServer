package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/webp"
)

type ExtSlice []string

func (list ExtSlice) Has(a string) bool {
	for _, b := range list {
		if strings.HasSuffix(a, b) {
			return true
		}
	}
	return false
}

var supported_types = ExtSlice{
	"jpg",
	"png",
	"gif",
	"webp",
	"jpeg",
	"svg",
}

var Config struct {
	Path     string
	Port     string
	Username string
	Password string
	Domain   string
}

func InitFlags() {
	Config.Path = os.Getenv("DATA_PATH")
	if Config.Path == "" {
		Config.Path = "./data"
	}

	Config.Port = os.Getenv("PORT")
	if Config.Port == "" {
		Config.Port = "5000"
	}

	Config.Username = os.Getenv("SERVER_USERNAME")
	if Config.Username == "" {
		Config.Username = "user"
	}

	Config.Password = os.Getenv("SERVER_PASSWORD")
	if Config.Password == "" {
		Config.Password = "test123"
	}

	Config.Domain = os.Getenv("DOMAIN")
	if Config.Domain == "" {
		Config.Domain = "https://image.mindustry-tool.app"
	}
}

func ContainsDotFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func BasicAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()

		if !ok || u != Config.Username || p != Config.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			log.Printf("%s %s\n%s %s\n", u, p, Config.Username, Config.Password)
			return
		}

		handler(w, r)
	}
}

type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"modTime"`
	IsDir    bool      `json:"isDir"`
}

func ListDirectory(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(Config.Path, r.URL.Path[1:])

	files, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	}

	var allFiles []FileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}
		if !ContainsDotFile(info.Name()) {
			allFiles = append(allFiles, FileInfo{
				Name:    info.Name(),
				Path:    filepath.Join(r.URL.Path, info.Name()),
				Size:    info.Size(),
				ModTime: info.ModTime(),
				IsDir:   info.IsDir(),
			})
		}
	}

	// Get page size from query parameter
	pageSize := 10 // Default page size
	if size := r.URL.Query().Get("size"); size != "" {
		if s, err := strconv.Atoi(size); err == nil && s > 0 {
			pageSize = s
		}
	}

	// Apply pagination
	start := 0
	end := len(allFiles)
	if end > pageSize {
		end = pageSize
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allFiles[start:end])
}

func CreateDirectory(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(Config.Path, r.URL.Path[1:])

	if err := os.MkdirAll(path, 0755); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func HandleRequest(dirname string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// Check if request is for directory listing
			if r.URL.Query().Get("list") == "true" {
				BasicAuth(ListDirectory).ServeHTTP(w, r)
				return
			}
			ServeImage(w, r)
		case http.MethodPost:
			// Check if request is for directory creation
			if r.URL.Query().Get("mkdir") == "true" {
				BasicAuth(CreateDirectory).ServeHTTP(w, r)
				return
			}
			BasicAuth(PostImage).ServeHTTP(w, r)
		case http.MethodDelete:
			BasicAuth(DeleteImage).ServeHTTP(w, r)
		default:
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	}
}

func ServeImage(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	path := r.URL.Path[1:]

	if format != "" && !supported_types.Has(format) {
		http.Error(w, "Unsupported format", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(Config.Path, path)
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	stats, err := file.Stat()
	if err != nil || stats.IsDir() || ContainsDotFile(file.Name()) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	extension := filepath.Ext(file.Name())[1:]

	// Set caching headers
	w.Header().Add("Cache-Control", "public, max-age=31536000")

	// Serve the file directly if no conversion is needed
	if format == "" || format == extension {
		http.ServeFile(w, r, filePath)
		return
	}

	// Set MIME type based on requested format
	switch format {
	case "png":
		w.Header().Add("Content-Type", "image/png")
	case "jpg", "jpeg":
		w.Header().Add("Content-Type", "image/jpeg")
	case "webp":
		w.Header().Add("Content-Type", "image/webp")
	default:
		http.Error(w, "Unsupported format", http.StatusBadRequest)
		return
	}

	image, err := ReadImage(file)
	if err != nil {
		log.Println("Error reading image:", err)
		http.Error(w, "Error reading image", http.StatusInternalServerError)
		return
	}

	// Encode and send the image in the requested format
	switch format {
	case "jpg", "jpeg":
		if err := jpeg.Encode(w, image, &jpeg.Options{Quality: 100}); err != nil {
			log.Println("Error encoding JPEG:", err)
			http.Error(w, "Error encoding JPEG", http.StatusInternalServerError)
		}
	case "png":
		if err := png.Encode(w, image); err != nil {
			log.Println("Error encoding PNG:", err)
			http.Error(w, "Error encoding PNG", http.StatusInternalServerError)
		}

	default:
		http.Error(w, "Unsupported format", http.StatusBadRequest)
	}
}

func ReadImage(file *os.File) (image.Image, error) {
	extension := filepath.Ext(file.Name())[1:]

	switch extension {
	case "png":
		return png.Decode(file)

	case "jpg", "jpeg":
		return jpeg.Decode(file)

	case "webp":
		return webp.Decode(file)

	default:
		return nil, errors.New("unsupported format: " + extension)
	}

}

func PostImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		log.Println(err.Error())
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	folder := r.FormValue("folder")
	id := r.FormValue("id")

	if folder == "" {
		http.Error(w, "Invalid folder", http.StatusInternalServerError)
		return
	}

	folderPath := filepath.Join(Config.Path, folder)

	err := os.MkdirAll(folderPath, 0755)

	if err != nil {
		http.Error(w, "Error creating folder: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fileUploaded, _, err := r.FormFile("file")
	if err != nil {
		log.Println("Error retrieving file: " + err.Error())
		http.Error(w, "Error retrieving file: "+err.Error(), http.StatusBadRequest)
		return
	}

	defer fileUploaded.Close()

	fileBytes, err := io.ReadAll(fileUploaded)

	buffer := bytes.Clone(fileBytes[0:512])

	contentType := http.DetectContentType(buffer)

	format := strings.Split(contentType, "/")[1]

	if format != "" && !supported_types.Has(format) {
		http.Error(w, "Unsupported format", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "Error reading uploaded file", http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(folderPath, id+"."+format)

	file, err := os.Create(filePath)

	if err != nil {
		http.Error(w, "Error creating file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	if _, err := file.Write(fileBytes); err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	baseURL, err := url.Parse(Config.Domain)
	if err != nil {
		http.Error(w, "Invalid domain configuration", http.StatusInternalServerError)
		return
	}

	baseURL.Path = path.Join(baseURL.Path, folder, id+"."+format)
	if _, err := w.Write([]byte(baseURL.String())); err != nil {
		http.Error(w, "Error writing response", http.StatusInternalServerError)
		return
	}
}

func DeleteImage(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(Config.Path, r.URL.Path[1:])

	// Get file info to check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Use RemoveAll for directories and Remove for files
	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			http.Error(w, "Error deleting directory: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err := os.Remove(path); err != nil {
			http.Error(w, "Error deleting file: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprintf(w, "Successfully deleted: %s", r.URL.Path[1:])
}

func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err == io.EOF, err
}

func main() {
	InitFlags()
	dirname, err := filepath.Abs(Config.Path)
	if err != nil {
		log.Fatalf("Could not get absolute path: %s\n", err)
	}

	dirPath := filepath.Dir(Config.Path)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Fatalf("Can not make dir %s %s\n", Config.Path, err)
	}

	log.Printf("Serving %s on port %s\n", dirname, Config.Port)

	if err := http.ListenAndServe(":"+Config.Port, HandleRequest(dirname)); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
