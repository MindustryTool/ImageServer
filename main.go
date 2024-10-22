package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

var (
	path = flag.String("path", ".", "path to the folder to serve. Defaults to the current folder")
	port = flag.String("port", "8080", "port to serve on. Defaults to 8080")
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
	fileSystem := DotFileHidingFileSystem{http.Dir(dirname)}
	
	http.Handle("/", http.FileServer(fileSystem))
	
	return http.ListenAndServe(":" + port, nil)
}


func main() {
	flag.Parse()

	dirname, err := filepath.Abs(*path)
	if err != nil {
		log.Fatalf("Could not get absolute path to directory: %s: %s", dirname, err.Error())
	}

	log.Printf("Serving %s on port %s", dirname, *port)

	err = Serve(dirname, *port)
	if err != nil {
		log.Fatalf("Could not serve directory: %s: %s", dirname, err.Error())
	}

}
