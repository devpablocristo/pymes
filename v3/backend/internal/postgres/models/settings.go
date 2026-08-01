// Package models contains connection settings owned by the PostgreSQL adapter.
package models

type Settings struct {
	DatabaseURL     string
	ApplicationName string
}
