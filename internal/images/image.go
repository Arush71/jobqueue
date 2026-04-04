package images

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func GetDecocdedImage(path string) (image.Image, string, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Println("Image path not found : ", path)
		return nil, "", err
	}
	defer func() {
		_ = file.Close()
	}()
	img, format, err := image.Decode(file)
	return img, format, err
}
func SaveImage(img image.Image, format string, inputPath string, quality int) (string, error) {
	ext := filepath.Ext(inputPath)
	fileP := strings.TrimSuffix(inputPath, ext)
	outputPath := fileP + "_output" + ext
	outFile, err := os.Create(outputPath)
	defer func() {
		_ = outFile.Close()
	}()

	if err != nil {
		log.Println("couldn't create the file , ", outputPath)
		return "", err
	}
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(outFile, img, &jpeg.Options{Quality: quality})
	case "png":
		err = png.Encode(outFile, img)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		log.Println("error trying to encode the image, ", outputPath)
		return "", err
	}
	return outputPath, nil
}
