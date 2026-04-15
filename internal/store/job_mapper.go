package store

import (
	"encoding/json"

	"github.com/Arush71/jobqueue/internal/jobs"
)

func ToDBParams(params jobs.ParamsT) (json.RawMessage, error) {
	return json.Marshal(params)
}

func FromDBParams(params json.RawMessage) (jobs.ParamsT, error) {
	var par jobs.ParamsT
	err := json.Unmarshal(params, &par)
	return par, err
}
