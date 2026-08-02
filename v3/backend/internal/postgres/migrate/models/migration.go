// Package models contains migration data owned by the migration adapter.
package models

type Migration struct {
	Name string
	SQL  string
}
