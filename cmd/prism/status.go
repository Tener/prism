package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/webvictim/prism/internal/logfile"
	"github.com/webvictim/prism/internal/state"
	"github.com/webvictim/prism/internal/tbot"
	"github.com/webvictim/prism/internal/tshwrap"
)

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	addGlobalFlags(fs)
	_ = fs.Parse(args)
	out := color.Output
	// Before the state file, because a service definition pointing at a
	// binary that is gone is exactly why there may be no session to
	// report.
	staleness := serviceStale()
	if staleness.stale() {
		style := warningColor
		if staleness.Breaks {
			style = failureColor
		}
		style.Fprintf(out, "prism: service definition is stale (%s)\n", staleness.Reason)
		proxyColor.Fprintln(out, "       `prism up` repairs it; `prism install` rewrites it now")
	}

	s, err := state.Load()
	if err != nil {
		return err
	}
	if s == nil {
		warningColor.Fprintln(out, "prism: no active session")
		return nil
	}
	logDir, _ := state.DaemonLogDir()
	statePath, _ := state.Path()

	alive := "DEAD"
	if isServiceManaged() && serviceIsActive() {
		alive = fmt.Sprintf("alive (%s)", serviceManagedLabel())
	} else if s.DaemonPID != 0 && processAlive(s.DaemonPID) {
		alive = fmt.Sprintf("alive (pid %d)", s.DaemonPID)
	} else if portResponds(s.LocalPort) {
		alive = "alive (port responding)"
	}

	headingColor.Fprintln(out, "prism status")
	dimColor.Fprintln(out, "------------")
	field := func(label, value string) {
		fmt.Fprintf(out, "%s %s\n", boldColor.Sprintf("%-12s", label), value)
	}
	aliveStyle := successColor
	if alive == "DEAD" {
		aliveStyle = failureColor
	}
	field("Router:", fmt.Sprintf("%s  %s", nameColor.Sprintf("127.0.0.1:%d", s.LocalPort), aliveStyle.Sprintf("[%s]", alive)))
	field("Health:", proxyColor.Sprintf("http://127.0.0.1:%d/_prism/health", s.LocalPort))
	field("Anthropic:", fmt.Sprintf("%s (app=anthropic)", nameColor.Sprintf("127.0.0.1:%d", s.AnthropicPort)))
	field("OpenAI:", fmt.Sprintf("%s (app=openai)", nameColor.Sprintf("127.0.0.1:%d", s.OpenAIPort)))
	if s.TbotDiagPort != 0 {
		field("Bot diag:", nameColor.Sprintf("127.0.0.1:%d", s.TbotDiagPort))
	}
	// `prism status` reports; `prism doctor` explains. Anything here
	// that isn't healthy earns a pointer at the command that says what
	// to do about it, since status itself offers no fixes.
	unhealthy := staleness.stale() || alive == "DEAD"
	if s.Identity() == state.IdentitySourceTbot {
		health := tbotHealthLine(s.TbotDiagPort)
		healthStyle := warningColor
		if strings.HasPrefix(health, "ok") {
			healthStyle = successColor
		} else if strings.HasPrefix(health, "unreachable") || strings.HasPrefix(health, "LIVE FAIL") {
			healthStyle = failureColor
		}
		field("Identity:", fmt.Sprintf("tbot (managed, dir=%s)", proxyColor.Sprint(s.TbotDir)))
		field("Bot health:", healthStyle.Sprint(health))
		if !strings.HasPrefix(health, "ok") {
			unhealthy = true
		}
		if msg := tbotStaleTTLWarning(s.TbotDir); msg != "" {
			warningColor.Fprintf(out, "\n  WARNING: %s\n", msg)
			unhealthy = true
		}
	} else {
		identity := identityLine()
		identityStyle := warningColor
		if strings.HasPrefix(identity, "ok") {
			identityStyle = successColor
		} else if strings.HasPrefix(identity, "EXPIRED") || strings.HasPrefix(identity, "not logged in") {
			identityStyle = failureColor
		}
		field("Identity:", identityStyle.Sprint(identity))
		if !strings.HasPrefix(identity, "ok") {
			unhealthy = true
		}
	}
	field("Created:", fmt.Sprintf("%s (%s ago)", s.CreatedAt.Local().Format(time.RFC3339), time.Since(s.CreatedAt).Round(time.Second)))
	field("State file:", proxyColor.Sprint(statePath))
	if isServiceManaged() && serviceManagedLabel() == "systemd-managed" {
		field("Daemon log:", proxyColor.Sprint("journalctl --user -u prism"))
	} else {
		field("Daemon log:", proxyColor.Sprint(logfile.LatestPath(logDir)))
	}
	if unhealthy {
		warningColor.Fprintln(out, "\nSomething above isn't healthy — `prism doctor` checks it all and says what to do.")
	}
	return nil
}

func tbotStaleTTLWarning(tbotDir string) string {
	if tbotDir == "" {
		return ""
	}
	yamlPath := tbot.TbotYAMLPath(tbotDir)
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return ""
	}
	contents := string(data)
	if strings.Contains(contents, "renewal_interval: 23h") || strings.Contains(contents, "renewal_interval: 24h") {
		return "tbot.yaml has a renewal_interval that exceeds the server's 12h cert lifetime.\n" +
			"           Certs will expire before renewal. Run `prism down && prism up` to fix."
	}
	return ""
}

func tbotHealthLine(diagPort int) string {
	if diagPort == 0 {
		return "unknown (no --diag-addr)"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	h := tbot.ProbeDiag(ctx, diagPort)
	switch {
	case !h.Reachable:
		return fmt.Sprintf("unreachable on 127.0.0.1:%d (%v)", diagPort, h.Err)
	case !h.Live:
		return fmt.Sprintf("LIVE FAIL on 127.0.0.1:%d/livez", diagPort)
	case !h.Ready:
		// The body is JSON tbot pretty-prints, and this is a one-line
		// field.
		return fmt.Sprintf("not ready: %s", oneLine(h.ReadyBody))
	case h.Degraded:
		return fmt.Sprintf("degraded (tunnel ok, but: %s)", h.DegradedReason)
	default:
		return fmt.Sprintf("ok (live + ready on 127.0.0.1:%d)", diagPort)
	}
}

func identityLine() string {
	st, err := tshwrap.StatusJSON()
	if err != nil {
		return fmt.Sprintf("unknown (tsh status failed: %v)", err)
	}
	login := "`tsh login`"
	if p := tshwrap.Proxy(); p != "" {
		login = "`tsh login --proxy=" + p + "`"
	}
	if !st.LoggedIn() {
		return "not logged in — run " + login
	}
	if st.IsExpired() {
		return fmt.Sprintf("EXPIRED at %s — run %s", st.ValidUntil.Local().Format(time.RFC3339), login)
	}
	remaining := time.Until(st.ValidUntil).Round(time.Minute)
	return fmt.Sprintf("ok as %s on %s (expires in %s)", st.Username, st.Cluster, remaining)
}

func portResponds(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
