package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"

	"io/fs"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

var (
	servePath = flag.String("path", ".", "path to the folder to serve. Defaults to the current folder")
	port      = flag.String("port", "8080", "port to serve on. Defaults to 8080")
	username  = os.Getenv("USERNAME")
	password  = os.Getenv("PASSWORD")
)

func ContainsDotFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

type DotFileHidingFileSystem struct {
	http.FileSystem
}

func (fileSystem DotFileHidingFileSystem) Open(name string) (http.File, error) {
	if ContainsDotFile(name) {
		return nil, fs.ErrPermission
	}

	file, err := fileSystem.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()

	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fs.ErrPermission
	}

	fmt.Println("Get file " + name)

	return file, err
}

func Serve(dirname string, port string) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fileSystem := DotFileHidingFileSystem{http.Dir(dirname)}
		switch r.Method {
		case http.MethodGet:
			http.FileServer(fileSystem).ServeHTTP(w, r)
		case http.MethodPost:
			BasicAuth(PostImage).ServeHTTP(w, r)
		case http.MethodDelete:
			BasicAuth(DeleteImage).ServeHTTP(w, r)
		default:
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	return http.ListenAndServe(":"+port, nil)
}

func BasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || !(u == username) || !(p == password) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func CleanURL(w http.ResponseWriter, urlPath string) string {
	parsedURL, err := url.Parse(urlPath)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return ""
	}
	return path.Clean(parsedURL.Path)
}

func PostImage(w http.ResponseWriter, r *http.Request) {
	urlPath := CleanURL(w, r.URL.Path)[1:]

	dirPath := filepath.Dir(urlPath)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		http.Error(w, "Error creating folders: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fileName := filepath.Base(urlPath)
	filePath := filepath.Join(dirPath, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Error creating the file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	err = r.ParseMultipartForm(100 << 20) // 100MB
	if err != nil {
		http.Error(w, "Error parsing multipart form", http.StatusBadRequest)
		return
	}

	fileUploaded, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer fileUploaded.Close()

	if _, err := io.Copy(file, fileUploaded); err != nil {
		http.Error(w, "Error saving the file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Successfully saved: "+r.URL.Path)
}

func DeleteImage(w http.ResponseWriter, r *http.Request) {
	urlPath := CleanURL(w, r.URL.Path)[1:]

	if _, err := os.Stat(urlPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	tempPath := urlPath + ".deleting"
	if err := os.Rename(urlPath, tempPath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error preparing file deletion: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	err := os.Remove(tempPath)
	if err != nil {
		http.Error(w, "Error deleting file: "+err.Error(), http.StatusBadRequest)
		return
	}

	dirPath := filepath.Dir(urlPath)
	for dirPath != "." {
		isEmpty, err := isDirEmpty(dirPath)
		if err != nil {
			http.Error(w, "Error checking directory: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if isEmpty {
			err = os.Remove(dirPath)
			if err != nil {
				http.Error(w, "Error deleting directory: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			break
		}
		dirPath = filepath.Dir(dirPath)
	}

	fmt.Fprintln(w, "Successfully deleted: "+urlPath)
}

func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func main() {
	flag.Parse()

	dirname, err := filepath.Abs(*servePath)
	if err != nil {
		log.Fatalf("Could not get absolute path to directory: %s: %s", dirname, err.Error())
	}

	log.Printf("Serving %s on port %s", dirname, *port)

	err = Serve(dirname, *port)
	if err != nil {
		log.Fatalf("Could not serve directory: %s: %s", dirname, err.Error())
	}

}
