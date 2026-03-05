package utils

import "encoding/json"

func CanonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
