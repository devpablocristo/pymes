package dto

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}
