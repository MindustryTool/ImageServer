package utils

import (
	"ImageServer/config"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"log"
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

func ReadImage(filePath string, variant string) (image.Image, error) {
	file, err := os.Open(filePath)
	if err != nil {
		file, err = os.Open(filePath + ".png")
		if err != nil {
			file, err = os.Open(filePath + ".jpg")
			if err != nil {
				file, err = os.Open(filePath + ".webp")
				if err != nil {
					file, err = os.Open(filePath + ".jpeg")
					if err != nil {
						return nil, errors.New("file not found")
					}
				}
			}
		}
	}
	
	defer file.Close()

	stats, err := file.Stat()
	if err != nil || stats.IsDir() || ContainsDotFile(file.Name()) {
		return nil, errors.New("file not found")
	}

	img, err := png.Decode(file)
	if err != nil {
		return nil, errors.New("error decoding image: " + err.Error())
	}

	if variant != "" {
		variantPath := filePath + "." + variant + filepath.Ext(filePath)
		img = ApplyVariant(img, variant)

		// Write image to variant path
		variantFile, err := os.Create(variantPath)
		if err != nil {
			return nil, errors.New("error creating variant file: " + err.Error())
		}
		defer variantFile.Close()
		if err := png.Encode(variantFile, img); err != nil {
			return nil, errors.New("error encoding variant image: " + err.Error())
		}
	}

	return img, nil
}

func Scale(img image.Image, size int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	var newW, newH int
	if srcW > srcH {
		newW = size
		newH = int(float64(srcH) * float64(size) / float64(srcW))
	} else {
		newH = size
		newW = int(float64(srcW) * float64(size) / float64(srcH))
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return dst
}

func ApplyVariant(img image.Image, variant string) image.Image {
	switch variant {
	case "preview":
		return Preview(img)
	default:
		return img
	}
}

func Preview(img image.Image) image.Image {
	// Preview does not exist, scale and write to disk
	previewImage := Scale(img, 256 + 128 )

	return previewImage
}

func FixAllFiles(cfg *config.Config) {
	baseDir, err := filepath.Abs(cfg.Path)
	if err != nil {
		log.Fatalf("Error getting absolute path: %v", err)
	}

	err = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Read image file
		ext := filepath.Ext(path)
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		var image image.Image
		
		switch ext {
			case ".png":
				image, err = png.Decode(file)
				if err != nil {
					return err
				}
			case ".jpg", ".jpeg":
				image, err = jpeg.Decode(file)
				if err != nil {
					return err
				}
			case "webp":
				image, err = webp.Decode(file)
				if err != nil {
					return err
				}
			default:
				return nil
		}

		filePathNoExt := path[:len(path)-len(filepath.Ext(path))]

		pngFile, err := os.Create(filePathNoExt)
		if err != nil {
			return err
		}
		defer pngFile.Close()
		if err := png.Encode(pngFile, image); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking path: %v", err)
	}
}
