// Package jobs handles jobs
package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/Arush71/jobqueue/internal/jobs/handler"
)

type registerType map[string]handler.JobHandler

// JobHandlerTypeHTTP is a type for unmarshelling the json data onto it.
type JobHandlerTypeHTTP struct {
	JobType  string          `json:"job_type"`
	Payload  json.RawMessage `json:"payload"`
	Priority string          `json:"priority"`
}

// Register map hooks up job handlers with their job type string.
var register = registerType{
	JobImageType: handler.ImageHandler{},
	JobFlakyType: handler.FlakyHandler{},
}

// GetJobHandler provides the functionality to verify and/or get the correct job handler
// for the job type.
func GetJobHandler(jobType string) (handler.JobHandler, error) {
	handler, ok := register[jobType]
	if !ok {
		return nil, fmt.Errorf("no such job type supported")
	}
	return handler, nil
}
