package mode

import "testing"

func TestString(t *testing.T) {
	tests := []struct {
		m    Mode
		want string
	}{
		{ModeProduction, "production"},
		{ModeDevelopment, "development"},
	}
	for _, tt := range tests {
		if got := tt.m.String(); got != tt.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tt.m, got, tt.want)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  Mode
	}{
		{"development", ModeDevelopment},
		{"Development", ModeDevelopment},
		{"DEVELOPMENT", ModeDevelopment},
		{"dev", ModeDevelopment},
		{"Dev", ModeDevelopment},
		{"develop", ModeDevelopment},
		{"  dev  ", ModeDevelopment},
		{"production", ModeProduction},
		{"Production", ModeProduction},
		{"PRODUCTION", ModeProduction},
		{"prod", ModeProduction},
		{"", ModeProduction},
		{"anything-else", ModeProduction},
	}
	for _, tt := range tests {
		got := Parse(tt.input)
		if got != tt.want {
			t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsDevelopment(t *testing.T) {
	if !ModeDevelopment.IsDevelopment() {
		t.Error("ModeDevelopment.IsDevelopment() should be true")
	}
	if ModeProduction.IsDevelopment() {
		t.Error("ModeProduction.IsDevelopment() should be false")
	}
}

func TestIsProduction(t *testing.T) {
	if !ModeProduction.IsProduction() {
		t.Error("ModeProduction.IsProduction() should be true")
	}
	if ModeDevelopment.IsProduction() {
		t.Error("ModeDevelopment.IsProduction() should be false")
	}
}

func TestParseRoundTrip(t *testing.T) {
	for _, m := range []Mode{ModeProduction, ModeDevelopment} {
		got := Parse(m.String())
		if got != m {
			t.Errorf("round-trip Parse(m.String()): got %v, want %v", got, m)
		}
	}
}
