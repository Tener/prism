package main

import (
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestColorModes(t *testing.T) {
	previous := color.NoColor
	t.Cleanup(func() {
		color.NoColor = previous
		_ = setColorMode("auto")
	})
	for _, tt := range []struct {
		mode    string
		noColor bool
		colored bool
	}{
		{"always", true, true},
		{"never", false, false},
		{"auto", true, false},
		{"auto", false, true},
	} {
		color.NoColor = tt.noColor
		if err := setColorMode(tt.mode); err != nil {
			t.Fatal(err)
		}
		for _, style := range colorStyles {
			if got := strings.Contains(style.Sprint("text"), "\x1b["); got != tt.colored {
				t.Fatalf("mode %q with NoColor=%v: colored=%v, want %v", tt.mode, tt.noColor, got, tt.colored)
			}
		}
	}
	if err := setColorMode("invalid"); err == nil {
		t.Fatal("accepted an invalid color mode")
	}
}
