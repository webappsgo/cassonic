package update

import (
	"log"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		release string
		want    bool
	}{
		{
			name:    "same version",
			current: "1.0.0",
			release: "1.0.0",
			want:    false,
		},
		{
			name:    "patch bump is newer",
			current: "1.0.0",
			release: "1.0.1",
			want:    true,
		},
		{
			name:    "minor bump is newer",
			current: "1.0.0",
			release: "1.1.0",
			want:    true,
		},
		{
			name:    "major bump is newer",
			current: "1.0.0",
			release: "2.0.0",
			want:    true,
		},
		{
			name:    "current is newer than release",
			current: "2.0.0",
			release: "1.9.9",
			want:    false,
		},
		{
			name:    "release equals current minor",
			current: "1.2.0",
			release: "1.2.0",
			want:    false,
		},
		{
			name:    "major older",
			current: "2.0.0",
			release: "1.0.0",
			want:    false,
		},
		{
			name:    "dev is treated as 0",
			current: "dev",
			release: "1.0.0",
			want:    true,
		},
		{
			name:    "current dev release dev",
			current: "dev",
			release: "dev",
			want:    false,
		},
		{
			name:    "v prefix stripped from release",
			current: "1.0.0",
			release: "v1.0.1",
			want:    true,
		},
		{
			name:    "v prefix stripped from both",
			current: "v1.0.0",
			release: "v1.0.0",
			want:    false,
		},
		{
			name:    "pre-release string treated as 0",
			current: "1.0.0",
			release: "1.0.0-beta",
			want:    false,
		},
		{
			name:    "multi-digit versions",
			current: "10.20.30",
			release: "10.20.31",
			want:    true,
		},
		{
			name:    "multi-digit current newer",
			current: "10.20.31",
			release: "10.20.30",
			want:    false,
		},
		{
			name:    "empty current treated as 0",
			current: "",
			release: "1.0.0",
			want:    true,
		},
		{
			name:    "empty release treated as 0",
			current: "1.0.0",
			release: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.current, log.Default())
			rel := &Release{Version: tt.release}
			got := c.IsNewer(rel)
			if got != tt.want {
				t.Errorf("IsNewer(current=%q, release=%q): got %v, want %v",
					tt.current, tt.release, got, tt.want)
			}
		})
	}
}

func TestSemverLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.1", "1.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"0.9.9", "1.0.0", true},
		{"1.0.0", "0.9.9", false},
		{"1.10.0", "1.9.0", false},
		{"1.9.0", "1.10.0", true},
	}

	for _, tt := range tests {
		got := semverLess(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("semverLess(%q, %q): got %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestVersionParts(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"v1.2.3", []int{1, 2, 3}},
		{"0.0.1", []int{0, 0, 1}},
		{"dev", []int{0}},
		{"", []int{0}},
		{"1", []int{1}},
		{"1.0", []int{1, 0}},
	}

	for _, tt := range tests {
		got := versionParts(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("versionParts(%q): len %d, want %d (got %v, want %v)",
				tt.input, len(got), len(tt.want), got, tt.want)
			continue
		}
		for i, v := range tt.want {
			if got[i] != v {
				t.Errorf("versionParts(%q)[%d]: got %d, want %d", tt.input, i, got[i], v)
			}
		}
	}
}
