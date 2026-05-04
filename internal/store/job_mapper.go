// Package store provides helpers for converting job parameters
// between in-memory structures and database representations.
package store

import (
	"encoding/json"

	"github.com/Arush71/jobqueue/internal/jobs"
)

// ToDBParams serializes job parameters into JSON for database storage.
func ToDBParams(params jobs.ParamsT) (json.RawMessage, error) {
	return json.Marshal(params)
}

// FromDBParams deserializes JSON data from the database into job parameters.
func FromDBParams(params json.RawMessage) (jobs.ParamsT, error) {
	var par jobs.ParamsT
	err := json.Unmarshal(params, &par)
	return par, err
}
