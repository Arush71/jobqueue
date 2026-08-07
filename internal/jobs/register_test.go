package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJobHandler(t *testing.T) {
	for _, jobType := range []string{JobImageType, JobFlakyType} {
		h, err := GetJobHandler(jobType)
		require.NoError(t, err)
		assert.NotNil(t, h)
	}
	_, err := GetJobHandler("missing")
	assert.Error(t, err)
}
