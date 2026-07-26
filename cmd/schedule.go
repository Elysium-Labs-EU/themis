package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Elysium-Labs-EU/themis/internal/binpath"
	"github.com/Elysium-Labs-EU/themis/internal/schedule"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

// runner runs an external command (systemctl/launchctl) and returns its
// combined output. Injected so the schedulers can be tested without a real
// init system; execRunner is the production implementation.
type runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// execRunner resolves name through binpath's trusted dirs (never $PATH,
// same as every other command themis spawns as root) and runs it.
func execRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	bin, err := binpath.Resolve(name)
	if err != nil {
		return nil, err
	}
	c := exec.CommandContext(ctx, bin, args...) // #nosec G204 -- bin resolved via binpath.Resolve from a fixed trusted-dir set
	c.Env = binpath.Environ(os.Environ())
	out, err := c.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out, nil
}

// scheduleStatus is a backend-agnostic snapshot of the installed unit: is
// it present on disk, does the init system consider it active, and a
// human-readable detail line (last/next run, or why it isn't active).
type scheduleStatus struct {
	Backend   string
	Detail    string
	Installed bool
	Active    bool
}

// scheduler installs, removes, and inspects the OS-native scheduled unit.
// One implementation per init system, selected by detectScheduler.
type scheduler interface {
	Enable(ctx context.Context, spec schedule.Spec) error
	Disable(ctx context.Context) error
	Status(ctx context.Context) (scheduleStatus, error)
	Backend() string
}

// detectScheduler picks the backend for this host: systemd when its
// runtime dir is present, launchd on macOS, cron.d as the Linux fallback.
// root is the filesystem root the unit dirs hang off, so tests can point
// the whole thing at a temp dir. An empty root means the real root "/" —
// normalized here because filepath.Join("", "etc/...") yields a *relative*
// path, which would probe run/systemd/system in the cwd and write unit
// files under it instead of the absolute system locations.
func detectScheduler(root string, run runner) (scheduler, error) {
	root = normalizeRoot(root)
	switch runtime.GOOS {
	case "linux":
		if _, err := os.Stat(filepath.Join(root, "run/systemd/system")); err == nil {
			return systemdScheduler{unitDir: filepath.Join(root, "etc/systemd/system"), run: run}, nil
		}
		return cronScheduler{cronDir: filepath.Join(root, "etc/cron.d")}, nil
	case "darwin":
		return launchdScheduler{daemonDir: filepath.Join(root, "Library/LaunchDaemons"), run: run}, nil
	default:
		return nil, &ui.UserError{Err: fmt.Errorf("themis schedule is not supported on %s (needs systemd, launchd, or cron)", runtime.GOOS)}
	}
}

// normalizeRoot maps an empty root to the real filesystem root "/". Without
// it filepath.Join("", "etc/systemd/system") returns a *relative*
// "etc/systemd/system", so detection would probe run/systemd/system under
// the cwd and unit files would land in a cwd-relative dir on a real host.
func normalizeRoot(root string) string {
	if root == "" {
		return "/"
	}
	return root
}

// --- systemd -------------------------------------------------------------

type systemdScheduler struct {
	run     runner
	unitDir string
}

func (s systemdScheduler) Backend() string { return "systemd" }

func (s systemdScheduler) servicePath() string { return filepath.Join(s.unitDir, schedule.ServiceUnit) }
func (s systemdScheduler) timerPath() string   { return filepath.Join(s.unitDir, schedule.TimerUnit) }

func (s systemdScheduler) Enable(ctx context.Context, spec schedule.Spec) error {
	service, err := schedule.SystemdService(spec)
	if err != nil {
		return err
	}
	timer, err := schedule.SystemdTimer(spec)
	if err != nil {
		return err
	}
	if err := writeUnitFile(s.servicePath(), service); err != nil {
		return err
	}
	if err := writeUnitFile(s.timerPath(), timer); err != nil {
		return err
	}
	if _, err := s.run(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, err := s.run(ctx, "systemctl", "enable", "--now", schedule.TimerUnit); err != nil {
		return err
	}
	return nil
}

func (s systemdScheduler) Disable(ctx context.Context) error {
	// Best-effort disable: a timer whose files were already removed (or
	// never enabled) makes systemctl exit non-zero, which must not block
	// removing the files themselves. The unit files are the source of truth.
	_, _ = s.run(ctx, "systemctl", "disable", "--now", schedule.TimerUnit)
	if err := removeIfPresent(s.timerPath()); err != nil {
		return err
	}
	if err := removeIfPresent(s.servicePath()); err != nil {
		return err
	}
	if _, err := s.run(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	return nil
}

func (s systemdScheduler) Status(ctx context.Context) (scheduleStatus, error) {
	installed := fileExists(s.servicePath()) && fileExists(s.timerPath())
	st := scheduleStatus{Backend: s.Backend(), Installed: installed}
	if !installed {
		st.Detail = "no themis.timer installed"
		return st, nil
	}
	// is-active exits non-zero for an inactive timer; that's a state, not a
	// failure, so the trimmed stdout ("active"/"inactive") is what matters.
	out, _ := s.run(ctx, "systemctl", "is-active", schedule.TimerUnit)
	st.Active = strings.TrimSpace(string(out)) == "active"
	if next, err := s.run(ctx, "systemctl", "show", schedule.TimerUnit, "--property=NextElapseUSecRealtime,LastTriggerUSec", "--value"); err == nil {
		st.Detail = strings.TrimSpace(strings.ReplaceAll(string(next), "\n", " "))
	}
	return st, nil
}

// --- launchd -------------------------------------------------------------

type launchdScheduler struct {
	run       runner
	daemonDir string
}

func (l launchdScheduler) Backend() string { return "launchd" }

func (l launchdScheduler) plistPath() string {
	return filepath.Join(l.daemonDir, schedule.LaunchdLabel+".plist")
}

func (l launchdScheduler) Enable(ctx context.Context, spec schedule.Spec) error {
	plist, err := schedule.LaunchdPlist(spec)
	if err != nil {
		return err
	}
	if err := writeUnitFile(l.plistPath(), plist); err != nil {
		return err
	}
	// bootstrap loads the job into the system domain; bootout first clears
	// any stale copy so a re-enable with changed schedule takes effect.
	_, _ = l.run(ctx, "launchctl", "bootout", "system", l.plistPath())
	if _, err := l.run(ctx, "launchctl", "bootstrap", "system", l.plistPath()); err != nil {
		return err
	}
	return nil
}

func (l launchdScheduler) Disable(ctx context.Context) error {
	_, _ = l.run(ctx, "launchctl", "bootout", "system", l.plistPath())
	return removeIfPresent(l.plistPath())
}

func (l launchdScheduler) Status(_ context.Context) (scheduleStatus, error) {
	installed := fileExists(l.plistPath())
	st := scheduleStatus{Backend: l.Backend(), Installed: installed, Active: installed}
	if installed {
		st.Detail = l.plistPath()
	} else {
		st.Detail = "no launchd agent installed"
	}
	return st, nil
}

// --- cron ----------------------------------------------------------------

type cronScheduler struct {
	cronDir string
}

func (c cronScheduler) Backend() string { return "cron" }

func (c cronScheduler) cronPath() string { return filepath.Join(c.cronDir, schedule.CronFile) }

func (c cronScheduler) Enable(_ context.Context, spec schedule.Spec) error {
	content, err := schedule.CronContent(spec)
	if err != nil {
		return err
	}
	return writeUnitFile(c.cronPath(), content)
}

func (c cronScheduler) Disable(_ context.Context) error {
	return removeIfPresent(c.cronPath())
}

func (c cronScheduler) Status(_ context.Context) (scheduleStatus, error) {
	installed := fileExists(c.cronPath())
	st := scheduleStatus{Backend: c.Backend(), Installed: installed, Active: installed}
	if installed {
		st.Detail = c.cronPath()
	} else {
		st.Detail = "no cron.d entry installed"
	}
	return st, nil
}

// --- shared effects ------------------------------------------------------

// writeUnitFile writes a generated unit file at 0600 owned by the current
// (root) user. systemd, launchd, and cron all run as root and read these
// directly, so they never need to be group/world readable — and cron in
// particular refuses /etc/cron.d files writable by anyone but their owner.
func writeUnitFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info == nil {
		return false
	}
	return !info.IsDir()
}

// --- command wiring ------------------------------------------------------

// resolveSpec builds the schedule.Spec an enable run installs: the running
// themis binary plus the operator's configured (or flag-overridden)
// interval and command. Defaults < config file < flags, mirroring check/apply.
func resolveSpec(cmd *cobra.Command) (schedule.Spec, error) {
	opCfg, err := loadOperatorConfig()
	if err != nil {
		return schedule.Spec{}, err
	}
	bin, err := currentBinaryPath()
	if err != nil {
		return schedule.Spec{}, err
	}
	interval := opCfg.Schedule.Interval
	if cmd.Flags().Changed("interval") {
		interval, _ = cmd.Flags().GetString("interval")
	}
	command := opCfg.Schedule.Command
	if cmd.Flags().Changed("command") {
		command, _ = cmd.Flags().GetString("command")
	}
	return schedule.Spec{BinaryPath: bin, Command: command, Interval: interval}, nil
}

func runScheduleEnable(cmd *cobra.Command, sched scheduler) error {
	spec, err := resolveSpec(cmd)
	if err != nil {
		return err
	}
	if err := sched.Enable(cmd.Context(), spec); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s scheduled themis %s (%s) via %s — interval %q\n",
		ui.LabelSuccess.Render("✓"), spec.Command, ui.TextMuted.Render("unattended"), sched.Backend(), spec.Interval)
	return nil
}

func runScheduleDisable(cmd *cobra.Command, sched scheduler) error {
	if err := sched.Disable(cmd.Context()); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s removed themis scheduled scan (%s)\n",
		ui.LabelSuccess.Render("✓"), sched.Backend())
	return nil
}

func runScheduleStatus(ctx context.Context, out io.Writer, sched scheduler) error {
	st, err := sched.Status(ctx)
	if err != nil {
		return err
	}
	printScheduleStatus(out, st)
	return nil
}

// printScheduleStatus renders a scheduleStatus. Pure formatting — no I/O
// beyond the passed writer.
func printScheduleStatus(out io.Writer, st scheduleStatus) {
	if !st.Installed {
		_, _ = fmt.Fprintf(out, "%s no themis scheduled scan installed (%s)\n",
			ui.TextMuted.Render("i"), st.Backend)
		if st.Detail != "" {
			_, _ = fmt.Fprintf(out, "  %s\n", ui.TextMuted.Render(st.Detail))
		}
		_, _ = fmt.Fprintf(out, "  run %s to install one\n", ui.TextCommand.Render("themis schedule enable"))
		return
	}
	state := ui.LabelWarning.Render("installed, inactive")
	if st.Active {
		state = ui.LabelSuccess.Render("installed, active")
	}
	_, _ = fmt.Fprintf(out, "%s themis scheduled scan %s (%s)\n", ui.LabelInfo.Render("i"), state, st.Backend)
	if st.Detail != "" {
		_, _ = fmt.Fprintf(out, "  %s\n", ui.TextMuted.Render(st.Detail))
	}
}

func newScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Install or remove a recurring themis scan (systemd timer / launchd / cron)",
		Long: `Install, remove, or inspect an OS-native recurring themis scan.

enable installs and activates a scheduled unit that runs ` + "`themis check`" + ` (or
` + "`themis apply --yes`" + `) on an interval; disable removes it; status reports
whether one is installed and when it last/next runs. The backend is chosen
per host: a systemd timer + oneshot service, a launchd StartCalendarInterval
daemon on macOS, or a cron.d entry as the Linux fallback.

The interval and command come from the schedule block of themis's config
file (defaults: interval daily, command check) and can be overridden with
--interval / --command.`,
	}

	enable := &cobra.Command{
		Use:   "enable",
		Short: "Install and activate the recurring scan",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireRoot("schedule enable", "install a system scheduled unit"); err != nil {
				return err
			}
			sched, err := detectScheduler("", execRunner)
			if err != nil {
				return err
			}
			return runScheduleEnable(cmd, sched)
		},
	}
	enable.Flags().String("interval", "", "override the configured interval (daily | weekly | systemd OnCalendar expr)")
	enable.Flags().String("command", "", "override the configured command to run (check | apply)")

	disable := &cobra.Command{
		Use:   "disable",
		Short: "Remove the recurring scan and its unit files",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireRoot("schedule disable", "remove the system scheduled unit"); err != nil {
				return err
			}
			sched, err := detectScheduler("", execRunner)
			if err != nil {
				return err
			}
			return runScheduleDisable(cmd, sched)
		},
	}

	status := &cobra.Command{
		Use:   "status",
		Short: "Report whether a recurring scan is installed and its state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sched, err := detectScheduler("", execRunner)
			if err != nil {
				return err
			}
			return runScheduleStatus(cmd.Context(), cmd.OutOrStdout(), sched)
		},
	}

	cmd.AddCommand(enable, disable, status)
	return cmd
}

var scheduleCmd = newScheduleCmd()
