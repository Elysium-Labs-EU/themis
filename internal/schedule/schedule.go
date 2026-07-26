// Package schedule builds the OS-native units that run themis on a
// recurring interval: a systemd timer + oneshot service, a launchd
// StartCalendarInterval agent, or a cron.d entry. Everything here is
// pure — it turns a resolved Spec into unit-file text and argv. Writing
// those files and driving systemctl/launchctl is the command layer's job
// (effects at the edges). The unit shapes deliberately mirror eos's
// system-unit generation so the two stay recognizably in sync, differing
// only where a scheduled oneshot must (Type=oneshot + a timer, not a
// long-running Type=simple daemon).
package schedule

import (
	"fmt"
	"strings"
)

// Unit identity, shared by the builders and the command layer so the file
// it writes and the systemctl/launchctl target it drives never drift apart.
const (
	// ServiceUnit is the oneshot systemd service the timer triggers.
	ServiceUnit = "themis.service"
	// TimerUnit is the systemd timer that schedules ServiceUnit.
	TimerUnit = "themis.timer"
	// LaunchdLabel is the launchd job label (and plist basename stem),
	// matching eos's org.elysiumlabs.* convention.
	LaunchdLabel = "org.elysiumlabs.themis"
	// CronFile is the basename themis writes under /etc/cron.d.
	CronFile = "themis"
)

// Spec is a fully resolved scheduled-scan request: the absolute themis
// binary to run, which subcommand ("check" or "apply"), and the operator's
// interval ("daily", "weekly", or a raw systemd OnCalendar expression).
// The command layer resolves these from config + the running binary path
// before handing a Spec to any builder.
type Spec struct {
	BinaryPath string
	Command    string
	Interval   string
}

// ExecArgs returns the themis argv (after the binary path) a scheduled run
// of command should invoke. "check" runs a plain audit; "apply" adds --yes
// so an unattended timer run never blocks on the interactive trust-network
// prompt (cmd/apply.go treats --yes as "apply with no exemption"). Any
// other command is rejected — the config only documents these two.
func ExecArgs(command string) ([]string, error) {
	switch command {
	case "check":
		return []string{"check"}, nil
	case "apply":
		return []string{"apply", "--yes"}, nil
	default:
		return nil, fmt.Errorf("unsupported schedule command %q: want \"check\" or \"apply\"", command)
	}
}

// SystemdOnCalendar maps an interval to a systemd OnCalendar value.
// "daily"/"weekly" are already valid OnCalendar shorthands; any other
// non-empty value is passed through verbatim as a raw OnCalendar
// expression (systemd validates it at unit load). An empty interval is an
// error rather than a unit that never fires.
func SystemdOnCalendar(interval string) (string, error) {
	if strings.TrimSpace(interval) == "" {
		return "", fmt.Errorf("empty schedule interval: want \"daily\", \"weekly\", or a systemd OnCalendar expression")
	}
	return interval, nil
}

// LaunchdCalendar is a launchd StartCalendarInterval: fire at Hour:Minute,
// restricted to Weekday when HasWeekday is set (0 = Sunday). launchd only
// takes calendar fields, so a raw systemd OnCalendar expression can't be
// expressed here — LaunchdCalendarFor rejects anything but daily/weekly.
type LaunchdCalendar struct {
	Weekday    int
	Hour       int
	Minute     int
	HasWeekday bool
}

// LaunchdCalendarFor translates a themis interval into a launchd calendar.
// Only "daily" and "weekly" translate; a raw OnCalendar expression is a
// systemd-only feature and is rejected here so the operator gets a clear
// error instead of a plist that never fires.
func LaunchdCalendarFor(interval string) (LaunchdCalendar, error) {
	switch interval {
	case "daily":
		return LaunchdCalendar{Hour: 0, Minute: 0}, nil
	case "weekly":
		return LaunchdCalendar{Weekday: 0, Hour: 0, Minute: 0, HasWeekday: true}, nil
	default:
		return LaunchdCalendar{}, fmt.Errorf("interval %q is not supported on launchd: use \"daily\" or \"weekly\" (raw OnCalendar expressions are systemd-only)", interval)
	}
}

// CronExprFor translates a themis interval into a standard 5-field cron
// expression. Like launchd, cron can't express a raw systemd OnCalendar,
// so only "daily" (midnight) and "weekly" (Sunday midnight) translate.
func CronExprFor(interval string) (string, error) {
	switch interval {
	case "daily":
		return "0 0 * * *", nil
	case "weekly":
		return "0 0 * * 0", nil
	default:
		return "", fmt.Errorf("interval %q is not supported on cron: use \"daily\" or \"weekly\" (raw OnCalendar expressions are systemd-only)", interval)
	}
}

// SystemdService renders the oneshot service the timer triggers. Type=oneshot
// (not eos's long-running Type=simple) because a scheduled scan runs to
// completion and exits; there is nothing to keep alive or restart.
func SystemdService(spec Spec) (string, error) {
	args, err := ExecArgs(spec.Command)
	if err != nil {
		return "", err
	}
	execStart := strings.Join(append([]string{spec.BinaryPath}, args...), " ")
	return fmt.Sprintf(`[Unit]
Description=themis scheduled hardening scan
After=network.target

[Service]
Type=oneshot
ExecStart=%s
`, execStart), nil
}

// SystemdTimer renders the timer that schedules SystemdService. Persistent=true
// runs a missed occurrence at next boot, so a host powered off at the
// scheduled time still scans once it comes back rather than silently
// skipping the interval.
func SystemdTimer(spec Spec) (string, error) {
	onCalendar, err := SystemdOnCalendar(spec.Interval)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`[Unit]
Description=themis scheduled hardening scan timer

[Timer]
OnCalendar=%s
Persistent=true
Unit=%s

[Install]
WantedBy=timers.target
`, onCalendar, ServiceUnit), nil
}

// LaunchdPlist renders the launchd LaunchDaemon plist. RunAtLoad is false
// so bootstrapping the job doesn't trigger an immediate scan — it only
// runs on the StartCalendarInterval schedule, matching the systemd timer's
// behavior.
func LaunchdPlist(spec Spec) (string, error) {
	args, err := ExecArgs(spec.Command)
	if err != nil {
		return "", err
	}
	cal, err := LaunchdCalendarFor(spec.Interval)
	if err != nil {
		return "", err
	}
	var argXML strings.Builder
	for _, a := range append([]string{spec.BinaryPath}, args...) {
		fmt.Fprintf(&argXML, "\t\t<string>%s</string>\n", a)
	}
	var calXML strings.Builder
	if cal.HasWeekday {
		fmt.Fprintf(&calXML, "\t\t<key>Weekday</key>\n\t\t<integer>%d</integer>\n", cal.Weekday)
	}
	fmt.Fprintf(&calXML, "\t\t<key>Hour</key>\n\t\t<integer>%d</integer>\n", cal.Hour)
	fmt.Fprintf(&calXML, "\t\t<key>Minute</key>\n\t\t<integer>%d</integer>", cal.Minute)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>StartCalendarInterval</key>
	<dict>
%s
	</dict>
	<key>RunAtLoad</key>
	<false/>
</dict>
</plist>
`, LaunchdLabel, argXML.String(), calXML.String()), nil
}

// CronContent renders an /etc/cron.d entry: the standard cron.d format is
// "m h dom mon dow user command", so it pins the run to root explicitly.
func CronContent(spec Spec) (string, error) {
	args, err := ExecArgs(spec.Command)
	if err != nil {
		return "", err
	}
	cronExpr, err := CronExprFor(spec.Interval)
	if err != nil {
		return "", err
	}
	command := strings.Join(append([]string{spec.BinaryPath}, args...), " ")
	return fmt.Sprintf(`# themis scheduled hardening scan — managed by `+"`themis schedule`"+`
%s root %s
`, cronExpr, command), nil
}
