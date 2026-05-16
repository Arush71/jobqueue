// Package handler handler adds job handlers and defines their usage
package handler

import "context"

// JobHandler interface defines the methods that every job handler will implement.
type JobHandler interface {
	Validate(payload []byte) error
	Process(ctx context.Context, payload []byte) ([]byte, error)
}
