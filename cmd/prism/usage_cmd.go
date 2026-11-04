package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"

	usagepkg "github.com/webvictim/prism/internal/usage"
)

func cmdUsage(args []string) error {
	fs := flag.NewFlagSet("usage", flag.ExitOnError)
	week := fs.Bool("week", false, "show last 7 days")
	all := fs.Bool("all", false, "show all time")
	asJSON := fs.Bool("json", false, "output raw JSONL records")
	addGlobalFlags(fs)
	_ = fs.Parse(args)

	path, err := usagepkg.Dir()
	if err != nil {
		return err
	}

	var since time.Time
	label := "today"
	switch {
	case *all:
		label = "all time"
	case *week:
		since = time.Now().AddDate(0, 0, -7).Truncate(24 * time.Hour)
		label = "last 7 days"
	default:
		since = time.Now().Truncate(24 * time.Hour)
	}

	records, err := usagepkg.Load(path, since)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		for _, r := range records {
			_ = enc.Encode(r)
		}
		return nil
	}

	out := color.Output
	if len(records) == 0 {
		fmt.Fprintf(out, "No usage data (%s)\n", label)
		return nil
	}

	// Totals.
	var totalIn, totalOut int64
	for _, r := range records {
		totalIn += r.InputTokens
		totalOut += r.OutputTokens
	}

	headingColor.Fprintf(out, "prism usage (%s, %d requests)\n", label, len(records))
	dimColor.Fprintln(out, repeat("─", 60))

	// By model.
	models := usagepkg.Summary(records)
	headingColor.Fprintf(out, "\n%-40s %8s %8s\n", "Model", "Input", "Output")
	dimColor.Fprintln(out, repeat("─", 60))
	for _, m := range models {
		name := m.Model
		if name == "" {
			name = "(unknown)"
		}
		fmt.Fprintf(out, "%s %s %s", nameColor.Sprintf("%-40s", name), inputColor.Sprintf("%8s", usagepkg.FormatTokens(m.InputTokens)), outputColor.Sprintf("%8s", usagepkg.FormatTokens(m.OutputTokens)))
		if m.CacheRead > 0 || m.CacheCreate > 0 {
			fmt.Fprintf(out, "  cache: r=%s w=%s", inputColor.Sprint(usagepkg.FormatTokens(m.CacheRead)), outputColor.Sprint(usagepkg.FormatTokens(m.CacheCreate)))
		}
		fmt.Fprintln(out)
	}

	// Models the gateway swapped. Its own section because the number
	// that matters is how often it happened, not the token counts: the
	// client reported (and priced) the model it asked for every time.
	if subs := usagepkg.Substitutions(records); len(subs) > 0 {
		warningColor.Fprintf(out, "\n%-49s %9s\n", "Substituted by gateway  [!]", "Requests")
		dimColor.Fprintln(out, repeat("─", 60))
		for _, sub := range subs {
			warningColor.Fprintf(out, "%-49s %9d\n", sub.Requested+" → "+sub.Served, sub.Requests)
		}
	}

	// By proxy.
	proxies := usagepkg.SummaryByProxy(records)
	if len(proxies) > 0 {
		headingColor.Fprintf(out, "\n%-40s %8s %8s\n", "Proxy", "Input", "Output")
		dimColor.Fprintln(out, repeat("─", 60))
		for _, p := range proxies {
			fmt.Fprintf(out, "%s %s %s\n", proxyColor.Sprintf("%-40s", p.Proxy), inputColor.Sprintf("%8s", usagepkg.FormatTokens(p.InputTokens)), outputColor.Sprintf("%8s", usagepkg.FormatTokens(p.OutputTokens)))
		}
	}

	dimColor.Fprintln(out, repeat("─", 60))
	successColor.Fprintf(out, "%-40s %8s %8s\n", "TOTAL", usagepkg.FormatTokens(totalIn), usagepkg.FormatTokens(totalOut))

	return nil
}

func repeat(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for range n {
		out = append(out, s...)
	}
	return string(out)
}
