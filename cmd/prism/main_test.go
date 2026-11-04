package main

import (
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestParseGlobalArgs(t *testing.T) {
	previous := color.NoColor
	color.NoColor = true
	t.Cleanup(func() {
		color.NoColor = previous
		_ = setColorMode("auto")
	})
	for _, tt := range []struct {
		name    string
		args    []string
		want    []string
		colored bool
	}{
		{"default", []string{"usage"}, []string{"usage"}, false},
		{"equals", []string{"--color=always", "usage", "--all"}, []string{"usage", "--all"}, true},
		{"split", []string{"--color", "always", "status"}, []string{"status"}, true},
		{"never", []string{"--color=never", "doctor"}, []string{"doctor"}, false},
		{"wrapped flags", []string{"--color=always", "codex", "--color=never", "--help"}, []string{"codex", "--color=never", "--help"}, true},
		{"exec flags", []string{"exec", "tool", "--color=always"}, []string{"exec", "tool", "--color=always"}, false},
		{"delimiter", []string{"--color=always", "--", "usage"}, []string{"usage"}, true},
		{"version", []string{"--version"}, []string{"version"}, false},
		{"short version", []string{"-v"}, []string{"version"}, false},
		{"help command", []string{"help"}, []string{"help"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGlobalArgs(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("args = %v, want %v", got, tt.want)
			}
			if colored := strings.Contains(headingColor.Sprint("text"), "\x1b["); colored != tt.colored {
				t.Fatalf("colored = %v, want %v", colored, tt.colored)
			}
		})
	}
	for _, alias := range []string{"--help", "-h"} {
		if _, err := parseGlobalArgs([]string{alias}); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("%s: error = %v, want ErrHelp", alias, err)
		}
	}
	if _, err := parseGlobalArgs([]string{"--color=invalid", "usage"}); err == nil {
		t.Fatal("accepted invalid color mode")
	}
}

func TestGlobalColorInheritance(t *testing.T) {
	t.Cleanup(func() { _ = setColorMode("auto") })
	for _, tt := range []struct {
		name    string
		args    []string
		colored bool
	}{
		{"inherit", nil, true},
		{"override", []string{"--color=never"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseGlobalArgs([]string{"--color=always", "usage"}); err != nil {
				t.Fatal(err)
			}
			fs := flag.NewFlagSet("usage", flag.ContinueOnError)
			addGlobalFlags(fs)
			if err := fs.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			if colored := strings.Contains(headingColor.Sprint("text"), "\x1b["); colored != tt.colored {
				t.Fatalf("colored = %v, want %v", colored, tt.colored)
			}
		})
	}
}
