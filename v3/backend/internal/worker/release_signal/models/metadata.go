// Package models contains data owned by the worker release-readiness adapter.
package models

type Metadata struct {
	ReleaseSHA string
	Revision   string
}
