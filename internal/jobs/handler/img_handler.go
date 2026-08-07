package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Arush71/jobqueue/internal/images"
	"github.com/disintegration/imaging"
)

// ImageHandler struct will implement the jobHandler interface and provide function for validating
// and processing images.
type ImageHandler struct{}

// JT represents the type of job to be performed.
type imageJobT string

// Supported job types.
const (
	Resize    imageJobT = "resize"
	GrayScale imageJobT = "grayscale"
	Compress  imageJobT = "compress"
)

type paramsT map[string]float64

type imagePayload struct {
	ImageJobType imageJobT `json:"image_job_type"`
	ImagePath    string    `json:"image_path"`
	Params       paramsT   `json:"params"`
}

func (payload *imagePayload) helpValidate() error {
	if strings.TrimSpace(payload.ImagePath) == "" {
		return fmt.Errorf("image path should not be empty")
	}
	switch payload.ImageJobType {
	case Resize:
		if v, ok := payload.Params["width"]; !ok || v <= 0 {
			return fmt.Errorf("must have width and be over 0")
		}
		if v, ok := payload.Params["height"]; !ok || v <= 0 {
			return fmt.Errorf("must have height and be over 0")
		}
		if len(payload.Params) > 2 {
			return fmt.Errorf("params must not have any extra fields")
		}
	case Compress:
		q, ok := payload.Params["quantity"]
		if !ok {
			return fmt.Errorf("must have quantity and be over 1 and under 100")
		}
		if q < 1 || q > 100 {
			return fmt.Errorf("must have quantity and be over 1 and under 100")
		}
		if len(payload.Params) > 1 {
			return fmt.Errorf("params must not have any extra fields")
		}
	case GrayScale:
		q, ok := payload.Params["quality"]
		if !ok {
			return fmt.Errorf("must have quality and be over 0.1 and under 1")
		}
		if q < 0.1 || q > 1 {
			return fmt.Errorf("must have quality and be over 0.1 and under 1")
		}
		if len(payload.Params) > 1 {
			return fmt.Errorf("params must not have any extra fields")
		}
	default:

		return fmt.Errorf("job type is required")
	}
	return nil
}

func (payload *imagePayload) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	jobType, ok := raw["image_job_type"].(string)
	if !ok {
		return fmt.Errorf("job type of image is required")
	}
	imagePath, ok := raw["image_path"].(string)
	if !ok {
		return fmt.Errorf("image path is required")
	}
	params, ok := raw["params"].(map[string]any)
	if !ok {
		return fmt.Errorf("params is required")
	}
	payload.ImagePath = imagePath
	jobTypeConversion := imageJobT(jobType)
	switch jobTypeConversion {
	case Resize, GrayScale, Compress:
		payload.ImageJobType = jobTypeConversion
	default:
		return fmt.Errorf("job image type is not valid")
	}
	payload.Params = make(paramsT)
	for k, v := range params {
		val, ok := v.(float64)
		if !ok {
			return fmt.Errorf("params field %s should be a number", k)
		}
		payload.Params[strings.ToLower(k)] = val
	}
	return nil
}

// Validate will validate payload for image handler.
func (ImageHandler) Validate(payload []byte) error {
	var img imagePayload
	err := json.Unmarshal(payload, &img)
	if err != nil {
		return err
	}
	if err := img.helpValidate(); err != nil {
		return err
	}
	return nil
}

type imageResult struct {
	OutputPath   string `json:"output_path"`
	OriginalPath string `json:"original_path"`
}

// Process will process the image for the image handler.
func (ImageHandler) Process(ctx context.Context, payload []byte) ([]byte, error) {
	type returnResults struct {
		Result []byte
		Failed error
	}
	done := make(chan returnResults, 1)

	go func() {
		var img imagePayload
		err := json.Unmarshal(payload, &img)
		if err != nil {
			done <- returnResults{Failed: err}
			return
		}
		// Always make sure that Validate runs before calling Process.
		// Validation done.
		image, format, err := images.GetDecodedImage(img.ImagePath)
		if err != nil {
			done <- returnResults{Failed: fmt.Errorf("error image process: couldn't either open or decode the image")}
			return
		}
		processedImg := image
		quality := 100
		switch img.ImageJobType {
		case Compress:
			quality = int(img.Params["quantity"])
		case GrayScale:
			processedImg = imaging.Grayscale(image)
		case Resize:
			processedImg = imaging.Resize(image, int(img.Params["width"]), int(img.Params["height"]), imaging.Lanczos)
		}
		outputPath, err := images.SaveImage(processedImg, format, img.ImagePath, quality)
		if err != nil {
			done <- returnResults{Failed: fmt.Errorf("error image process: couldn't save image")}
			return
		}
		imgRes := imageResult{
			OutputPath:   outputPath,
			OriginalPath: img.ImagePath,
		}
		data, err := json.Marshal(imgRes)
		if err != nil {
			done <- returnResults{Failed: err}
			return
		}
		done <- returnResults{Failed: nil, Result: data}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("job timeout")
	case result := <-done:
		return result.Result, result.Failed
	}
}
