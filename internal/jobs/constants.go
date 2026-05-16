package jobs

// MaxRetries is the maximum reties allowed for a job
const MaxRetries int16 = 3

const (
	// JobImageType is the string of image processing handler
	JobImageType string = "image.processing"
	// JobFlakyType is the string of flaky job handler
	JobFlakyType string = "flaky"
)
