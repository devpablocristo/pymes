// Package helpers contains environment codecs for the service-token adapter.
package helpers

import "strings"

// CSVValues parses optional key-version lists without empty entries.
func CSVValues(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
