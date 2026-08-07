package images

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndDecodeImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.White)
	for _, format := range []string{"jpeg", "png"} {
		t.Run(format, func(t *testing.T) {
			ext := "." + format
			input := filepath.Join(t.TempDir(), "source"+ext)
			output, err := SaveImage(img, format, input, 80)
			require.NoError(t, err)
			assert.FileExists(t, output)
			decoded, gotFormat, err := GetDecodedImage(output)
			require.NoError(t, err)
			assert.Equal(t, format, gotFormat)
			assert.Equal(t, img.Bounds(), decoded.Bounds())
		})
	}
}

func TestImageErrors(t *testing.T) {
	_, _, err := GetDecodedImage(filepath.Join(t.TempDir(), "missing.png"))
	assert.Error(t, err)

	bad := filepath.Join(t.TempDir(), "bad.png")
	require.NoError(t, os.WriteFile(bad, []byte("not an image"), 0o600))
	_, _, err = GetDecodedImage(bad)
	assert.Error(t, err)

	_, err = SaveImage(image.NewRGBA(image.Rect(0, 0, 1, 1)), "gif", filepath.Join(t.TempDir(), "x.gif"), 100)
	assert.Error(t, err)
}
