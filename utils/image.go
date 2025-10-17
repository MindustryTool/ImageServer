package utils

import (
	"ImageServer/config"
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

func FindImage(filePath string) (*os.File, error) {
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
						return nil, nil
					}
				}
			}
		}
	}

	return file, nil
}

// ReadImage loads an image from disk and applies a variant if specified.
// If the variant already exists, it is returned directly (cached).
func ReadImage(filePath, variant, ext string) (image.Image, error) {
	// 2. Load original image (with FindImage fallback: .png, .jpg, .webp, .jpeg)
	img, err := loadImage(filePath)
	if err != nil {
		println(err.Error())
		return nil, err
	}


	if img == nil {
		println("Image not found: " + filePath)
		return nil, nil
	}

	// 3. Apply variant and cache if requested
	if variant != "" {
		img = ApplyVariant(img, variant)
		variantPath := filePath + "." + variant + "." + ext
	
		if err := save(variantPath, img, ext); err != nil {
			println(err.Error())
			return nil, err
		}
	}

	return img, nil
}

// loadImage uses FindImage to open a file and decode it.
func loadImage(path string) (image.Image, error) {
	file, err := FindImage(path)
	if err != nil {
		println(err.Error())
		return nil, err
	}
	defer file.Close()

	if file == nil {
		println("File not found: " + path)
		return nil, nil
	}

	img, _, err := image.Decode(file)
	
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return img, nil
}

// save saves an image as PNG.
func save(path string, img image.Image, ext string) error {
	f, err := os.Create(path + "." + ext)
	if err != nil {
		return err
	}
	defer f.Close()

	println("Save image: " + path + "." + ext)

	switch ext {
		case "png":
			return png.Encode(f, img)
		case "jpg", "jpeg":
			return jpeg.Encode(f, img, nil)
		// case ".webp":
		// 	return webp.Encode(f, img, nil)
		default:
			return nil
	}
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
	previewImage := Scale(img, 256)

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
