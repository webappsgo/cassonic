package mode

import "strings"

// Mode represents the server operating mode.
type Mode int

const (
	ModeProduction  Mode = iota
	ModeDevelopment
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	if m == ModeDevelopment {
		return "development"
	}
	return "production"
}

// Parse parses a string into a Mode (case-insensitive).
// "development", "dev", and "develop" map to ModeDevelopment.
// Any other value maps to ModeProduction.
func Parse(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "development", "dev", "develop":
		return ModeDevelopment
	default:
		return ModeProduction
	}
}

// IsDevelopment returns true if the mode is development.
func (m Mode) IsDevelopment() bool {
	return m == ModeDevelopment
}

// IsProduction returns true if the mode is production.
func (m Mode) IsProduction() bool {
	return m == ModeProduction
}
