package main

import (
	"flag"
	"fmt"

	"github.com/fatih/color"
)

var (
	headingColor = color.New(color.Bold)
	dimColor     = color.New(color.Faint)
	nameColor    = color.New(color.FgHiCyan)
	proxyColor   = color.New(color.FgHiBlue)
	inputColor   = color.New(color.FgCyan)
	outputColor  = color.New(color.FgMagenta)
	successColor = color.New(color.FgGreen, color.Bold)
	warningColor = color.New(color.FgYellow, color.Bold)
	failureColor = color.New(color.FgRed, color.Bold)
	boldColor    = color.New(color.Bold)
	colorStyles  = []*color.Color{headingColor, dimColor, nameColor, proxyColor, inputColor, outputColor, successColor, warningColor, failureColor, boldColor}
)

type colorModeValue string

func (m *colorModeValue) String() string { return string(*m) }
func (m *colorModeValue) Get() any       { return string(*m) }

func (m *colorModeValue) Set(value string) error {
	if err := setColorMode(value); err != nil {
		return err
	}
	*m = colorModeValue(value)
	return nil
}

func addGlobalFlags(fs *flag.FlagSet) {
	mode := colorModeValue("auto")
	fs.Var(&mode, "color", "color output: auto, always, or never")
}

func setColorMode(mode string) error {
	var enabled bool
	switch mode {
	case "auto":
		enabled = !color.NoColor
	case "always":
		enabled = true
	case "never":
		enabled = false
	default:
		return fmt.Errorf("invalid color mode %q (use auto/always/never)", mode)
	}
	for _, style := range colorStyles {
		if enabled {
			style.EnableColor()
		} else {
			style.DisableColor()
		}
	}
	return nil
}
