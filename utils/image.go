package utils

import (
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
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

func Scale(img image.Image, width, height int) image.Image {
	scaledImage := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.BiLinear.Scale(scaledImage, scaledImage.Bounds(), img, img.Bounds(), draw.Over, nil)
	return scaledImage
}

func ApplyVariant(filePath string, img image.Image, variant string) image.Image {
	switch variant {
	case "preview":
		return Preview(filePath, img)
	default:
		return img
	}
}

func Preview(filePath string, img image.Image) image.Image {
	previewPath := filePath + ".preview" + filepath.Ext(filePath)

	if _, err := os.Stat(previewPath); err == nil {
		// Preview exists, read and return
		file, err := os.Open(previewPath)
		if err != nil {
			return img
		}
		defer file.Close()

		previewImage, err := ReadImage(file)
		if err != nil {
			return img
		}
		return previewImage
	}

	// Preview does not exist, scale and write to disk
	previewImage := Scale(img, 256, 256)

	file, err := os.Create(previewPath)
	if err != nil {
		return img
	}
	defer file.Close()

	if err := png.Encode(file, previewImage); err != nil {
		return img
	}

	return previewImage
}