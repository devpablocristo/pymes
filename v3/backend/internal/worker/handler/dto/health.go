// Package dto contains HTTP representations owned by the worker handler.
package dto

type Health struct {
	Status string `json:"status"`
}
