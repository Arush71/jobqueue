package helpers

import (
	"bytes"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestReadJSON(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"name":"job"}`))
	require.NoError(t, ReadJson(req, &dst, quietLogger()))
	assert.Equal(t, "job", dst.Name)

	req = httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"unknown":true}`))
	assert.Error(t, ReadJson(req, &dst, quietLogger()))

	req = httptest.NewRequest("POST", "/", bytes.NewBufferString(`{`))
	assert.Error(t, ReadJson(req, &dst, quietLogger()))
}

func TestJSONResponses(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteJson(recorder, 201, map[string]int{"id": 7})
	assert.Equal(t, 201, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"id":7}`, recorder.Body.String())

	recorder = httptest.NewRecorder()
	Error(recorder, 400, "bad")
	assert.Equal(t, 400, recorder.Code)
	assert.JSONEq(t, `{"error":"bad"}`, recorder.Body.String())
}
