package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type ExtSlice []string

func (list ExtSlice) Has(a string) bool {
    for _, b := range list {
        if (strings.HasSuffix(b, a)) {
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
}

func InitFlags() {
	flag.StringVar(&Config.Path, "PATH", ".", "Path to the folder to serve. Defaults to the current folder")
	flag.StringVar(&Config.Port, "PORT", "5000", "Port to serve on. Defaults to 5000")
	flag.StringVar(&Config.Username, "USERNAME", "admin", "Username")
	flag.StringVar(&Config.Password, "PASSWORD", "password", "Password")
	flag.Parse()
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

type DotFileHidingFileSystem struct {
	http.FileSystem
}

func (fs DotFileHidingFileSystem) Open(name string) (http.File, error) {
	if ContainsDotFile(name) {
		return nil, os.ErrPermission
	}
	file, err := fs.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	
	stat, _ := file.Stat();
	
	if stat.IsDir() {
		return nil, os.ErrPermission
	}

	if !supported_types.Has(stat.Name()) {
		return nil, os.ErrPermission
	}


	return file, nil
}

func BasicAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != Config.Username || p != Config.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}
}

func HandleRequest(dirname string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

	if (folder == ""){
		http.Error(w, "Invalid folder", http.StatusInternalServerError)
	}
	
	folderPath := filepath.Join(Config.Path, folder)
	
 	err :=	os.MkdirAll(folderPath, 0755)

	if err != nil {
		http.Error(w, "Error creating folder: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(folderPath, id + "." + format)

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

	if _, err := io.Copy(file, fileUploaded); err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Successfully saved: %s", filePath)
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
	
	if Config.Path != "." {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			log.Fatalf("Can not make dir %s %s\n", Config.Path, err)
		}
	}

	
	log.Printf("Serving %s on port %s\n", dirname, Config.Port)


	if err := http.ListenAndServe(":"+Config.Port, HandleRequest(dirname)); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
