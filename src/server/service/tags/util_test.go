package tags

import "testing"

func TestFieldString(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"int64", int64(123456789012), "123456789012"},
		{"float64", 3.14, "3.14"},
		{"float32", float32(2.5), "2.5"},
		{"bool_default", true, "true"},
		{"nil_default", nil, "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldString(tt.input)
			if got != tt.want {
				t.Errorf("fieldString(%v): got %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
