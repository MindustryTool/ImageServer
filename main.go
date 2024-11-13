package main

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
		Config.Username = "admin"
	}

	Config.Password = os.Getenv("SERVER_PASSWORD")
	if Config.Password == "" {
		Config.Password = "password"
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

func HandleRequest(dirname string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ServeImage(w, r)
		case http.MethodPost:
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

	// Serve the file directly if no conversion is needed
	if format == "" || format == extension {
		http.ServeFile(w, r, filePath)
		return
	}

	// Set MIME type based on requested format
	switch format {
	case "png":
		w.Header().Set("Content-Type", "image/png")
	case "jpg", "jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case "webp":
		w.Header().Set("Content-Type", "image/webp")
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

	// Set caching headers
	w.Header().Set("Expires", time.Now().AddDate(60, 0, 0).Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "public, max-age=31536000")

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
	format := r.FormValue("format")

	if folder == "" {
		http.Error(w, "Invalid folder", http.StatusInternalServerError)
		return
	}

	if supported_types.Has(folder) {
		http.Error(w, "Supported format", http.StatusBadRequest)
		return
	}

	folderPath := filepath.Join(Config.Path, folder)

	err := os.MkdirAll(folderPath, 0755)

	if err != nil {
		http.Error(w, "Error creating folder: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(folderPath, id+"."+format)

	file, err := os.Create(filePath)

	if err != nil {
		http.Error(w, "Error creating file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	fileUploaded, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}

	defer fileUploaded.Close()

	fileBytes, err := io.ReadAll(fileUploaded)

	if err != nil {
		http.Error(w, "Error reading uploaded file", http.StatusInternalServerError)
		return
	}

	if _, err := file.Write(fileBytes); err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Successfully saved: %s", filePath)

	w.Write([]byte(filepath.Join(Config.Domain, filePath)))
}

func DeleteImage(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Path[1:]
	if err := os.Remove(filePath); err != nil {
		http.Error(w, "Error deleting file: "+err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Fprintf(w, "Successfully deleted: %s", filePath)
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
