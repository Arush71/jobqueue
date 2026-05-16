package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

type flakyPayload struct {
	FailRate float64 `json:"fail_rate"`
	DelaySec int     `json:"delay_sec"` // simulate work duration
}

type flakyResult struct {
	Status string `json:"status"`
}

// FlakyHandler implements jobHandler
type FlakyHandler struct{}

// Validate validates the payload for flaky
func (FlakyHandler) Validate(payload []byte) error {
	var p flakyPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.FailRate < 0 || p.FailRate > 1 {
		return fmt.Errorf("fail_rate must be between 0 and 1")
	}
	if p.DelaySec < 0 {
		return fmt.Errorf("delay_ms must be between 0 and 10000")
	}
	return nil
}

// Process processes the payload for flaky
func (FlakyHandler) Process(ctx context.Context, payload []byte) ([]byte, error) {
	var p flakyPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}

	// simulate work with cancellation support
	select {
	case <-time.After(time.Duration(p.DelaySec) * time.Second):
	case <-ctx.Done():
		return nil, fmt.Errorf("job timeout")
	}

	// random failure
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if r.Float64() < p.FailRate {
		return nil, fmt.Errorf("simulated random failure")
	}

	res := flakyResult{Status: "success"}
	return json.Marshal(res)
}
