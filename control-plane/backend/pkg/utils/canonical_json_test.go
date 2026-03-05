package utils

import (
	"testing"
)

func TestCanonicalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    string
		wantErr bool
	}{
		{"nil", nil, "null", false},
		{"string", "hello", `"hello"`, false},
		{"number", 42, "42", false},
		{"map", map[string]string{"b": "2", "a": "1"}, `{"a":"1","b":"2"}`, false},
		{"nested", map[string]any{"key": []int{1, 2}}, `{"key":[1,2]}`, false},
		{"empty map", map[string]any{}, "{}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CanonicalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if string(got) != tt.want {
				t.Errorf("CanonicalJSON() = %s; want %s", got, tt.want)
			}
		})
	}
}
