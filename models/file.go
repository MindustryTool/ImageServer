package models

import (
	"strings"
	"time"
)

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
}

type ExtSlice []string

func (list ExtSlice) Has(a string) bool {
	for _, b := range list {
		if strings.HasSuffix(a, b) {
			return true
		}
	}
	return false
}

var SupportedTypes = ExtSlice{
	"jpg",
	"png",
	"gif",
	"webp",
	"jpeg",
	"svg",
}