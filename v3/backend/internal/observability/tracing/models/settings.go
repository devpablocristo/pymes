// Package models contains deployment settings owned by the tracing adapter.
package models

type Settings struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Exporter       string
	OTLPEndpoint   string
	OTLPInsecure   bool
	SampleRatio    float64
}
