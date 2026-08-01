// Package dto contains startup input owned by the fake-service handler.
package dto

type Kind string

const (
	Accounting Kind = "accounting"
	Fiscal     Kind = "fiscal"
)
