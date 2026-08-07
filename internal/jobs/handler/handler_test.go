package handler

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeJPEG(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := range 6 {
		for x := range 8 {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 100, A: 255})
		}
	}
	require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 90}))
	require.NoError(t, f.Close())
}

func imagePayloadJSON(t *testing.T, kind, path string, params map[string]float64) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"image_job_type": kind,
		"image_path":     path,
		"params":         params,
	})
	require.NoError(t, err)
	return b
}

func TestImageHandlerValidation(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		valid   bool
	}{
		{"resize", `{"image_job_type":"resize","image_path":"a.jpg","params":{"width":10,"height":20}}`, true},
		{"compress", `{"image_job_type":"compress","image_path":"a.jpg","params":{"quantity":75}}`, true},
		{"grayscale", `{"image_job_type":"grayscale","image_path":"a.jpg","params":{"quality":0.5}}`, true},
		{"bad json", `{`, false},
		{"missing type", `{"image_path":"a.jpg","params":{}}`, false},
		{"unknown type", `{"image_job_type":"rotate","image_path":"a.jpg","params":{}}`, false},
		{"blank path", `{"image_job_type":"resize","image_path":" ","params":{"width":10,"height":20}}`, false},
		{"missing width", `{"image_job_type":"resize","image_path":"a.jpg","params":{"height":20}}`, false},
		{"zero height", `{"image_job_type":"resize","image_path":"a.jpg","params":{"width":10,"height":0}}`, false},
		{"resize extra", `{"image_job_type":"resize","image_path":"a.jpg","params":{"width":10,"height":20,"x":1}}`, false},
		{"bad quantity", `{"image_job_type":"compress","image_path":"a.jpg","params":{"quantity":101}}`, false},
		{"compress extra", `{"image_job_type":"compress","image_path":"a.jpg","params":{"quantity":80,"x":1}}`, false},
		{"bad quality", `{"image_job_type":"grayscale","image_path":"a.jpg","params":{"quality":0.01}}`, false},
		{"non-number param", `{"image_job_type":"resize","image_path":"a.jpg","params":{"width":"wide","height":20}}`, false},
		{"missing params", `{"image_job_type":"resize","image_path":"a.jpg"}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := (ImageHandler{}).Validate([]byte(tc.payload))
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestImageHandlerProcessesOperations(t *testing.T) {
	for _, tc := range []struct {
		kind   string
		params map[string]float64
		width  int
		height int
	}{{"resize", map[string]float64{"width": 4, "height": 3}, 4, 3},
		{"compress", map[string]float64{"quantity": 50}, 8, 6},
		{"grayscale", map[string]float64{"quality": 0.5}, 8, 6}} {
		t.Run(tc.kind, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), tc.kind+".jpg")
			writeJPEG(t, input)
			payload := imagePayloadJSON(t, tc.kind, input, tc.params)
			result, err := (ImageHandler{}).Process(context.Background(), payload)
			require.NoError(t, err)
			var decoded imageResult
			require.NoError(t, json.Unmarshal(result, &decoded))
			assert.Equal(t, input, decoded.OriginalPath)
			out, err := os.Open(decoded.OutputPath)
			require.NoError(t, err)
			defer out.Close()
			img, err := jpeg.Decode(out)
			require.NoError(t, err)
			assert.Equal(t, tc.width, img.Bounds().Dx())
			assert.Equal(t, tc.height, img.Bounds().Dy())
		})
	}
}

func TestImageHandlerErrorsAndCancellation(t *testing.T) {
	missing := imagePayloadJSON(t, "resize", filepath.Join(t.TempDir(), "missing.jpg"), map[string]float64{"width": 1, "height": 1})
	_, err := (ImageHandler{}).Process(context.Background(), missing)
	assert.Error(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = (ImageHandler{}).Process(ctx, missing)
	assert.Error(t, err)
}

func TestFlakyHandler(t *testing.T) {
	h := FlakyHandler{}
	for _, tc := range []struct {
		payload string
		valid   bool
	}{{`{"fail_rate":0,"delay_sec":0}`, true}, {`{"fail_rate":-1,"delay_sec":0}`, false},
		{`{"fail_rate":2,"delay_sec":0}`, false}, {`{"fail_rate":0,"delay_sec":-1}`, false}, {`{`, false}} {
		err := h.Validate([]byte(tc.payload))
		assert.Equal(t, tc.valid, err == nil)
	}

	result, err := h.Process(context.Background(), []byte(`{"fail_rate":0,"delay_sec":0}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"success"}`, string(result))
	_, err = h.Process(context.Background(), []byte(`{"fail_rate":1,"delay_sec":0}`))
	assert.Error(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = h.Process(ctx, []byte(`{"fail_rate":0,"delay_sec":1}`))
	assert.Error(t, err)
}

func TestFlakyHandlerSpecialPanic(t *testing.T) {
	assert.Panics(t, func() {
		_, _ = (FlakyHandler{}).Process(context.Background(), []byte(`{"fail_rate":0.2,"delay_sec":0}`))
	})
}
