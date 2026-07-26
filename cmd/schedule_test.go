package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/schedule"
)

// recordingRunner captures every systemctl/launchctl invocation and returns
// canned stdout keyed by a substring of the argument list, so scheduler
// effects can be exercised with no real init system.
type recordingRunner struct {
	failWith error
	respond  map[string]string
	failOn   string
	calls    []string
}

func (r *recordingRunner) run(_ context.Context, name string, args ...string) ([]byte, error) {
	line := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, line)
	if r.failOn != "" && strings.Contains(line, r.failOn) {
		return nil, r.failWith
	}
	for sub, out := range r.respond {
		if strings.Contains(line, sub) {
			return []byte(out), nil
		}
	}
	return nil, nil
}

func (r *recordingRunner) calledWith(sub string) bool {
	for _, c := range r.calls {
		if strings.Contains(c, sub) {
			return true
		}
	}
	return false
}

func dailyCheckSpec(bin string) schedule.Spec {
	return schedule.Spec{BinaryPath: bin, Command: "check", Interval: "daily"}
}

func TestSystemdEnableWritesUnitsAndActivates(t *testing.T) {
	dir := t.TempDir()
	rr := &recordingRunner{}
	s := systemdScheduler{unitDir: dir, run: rr.run}

	if err := s.Enable(context.Background(), dailyCheckSpec("/usr/local/bin/themis")); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	svc := filepath.Join(dir, schedule.ServiceUnit)
	tim := filepath.Join(dir, schedule.TimerUnit)
	if !fileExists(svc) || !fileExists(tim) {
		t.Fatalf("expected both %s and %s written", schedule.ServiceUnit, schedule.TimerUnit)
	}
	body, _ := os.ReadFile(tim)
	if !strings.Contains(string(body), "OnCalendar=daily") {
		t.Errorf("timer missing OnCalendar=daily:\n%s", body)
	}
	if !rr.calledWith("systemctl daemon-reload") {
		t.Error("expected systemctl daemon-reload")
	}
	if !rr.calledWith("systemctl enable --now themis.timer") {
		t.Errorf("expected enable --now themis.timer, got %v", rr.calls)
	}
}

func TestSystemdEnableUnitFilePerms(t *testing.T) {
	dir := t.TempDir()
	s := systemdScheduler{unitDir: dir, run: (&recordingRunner{}).run}
	if err := s.Enable(context.Background(), dailyCheckSpec("/x/themis")); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, schedule.ServiceUnit))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("service perm = %o, want 600", perm)
	}
}

func TestSystemdStatusReflectsInstalledAndActive(t *testing.T) {
	dir := t.TempDir()
	rr := &recordingRunner{respond: map[string]string{"is-active": "active\n", "show": "0 12345\n"}}
	s := systemdScheduler{unitDir: dir, run: rr.run}

	// Before install: not installed.
	st, err := s.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Installed || st.Active {
		t.Errorf("pre-install status = %+v, want not installed", st)
	}

	if enableErr := s.Enable(context.Background(), dailyCheckSpec("/x/themis")); enableErr != nil {
		t.Fatalf("Enable: %v", enableErr)
	}
	st, err = s.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Installed || !st.Active {
		t.Errorf("post-install status = %+v, want installed+active", st)
	}
}

func TestSystemdDisableRemovesUnitsAndReloads(t *testing.T) {
	dir := t.TempDir()
	rr := &recordingRunner{}
	s := systemdScheduler{unitDir: dir, run: rr.run}
	if err := s.Enable(context.Background(), dailyCheckSpec("/x/themis")); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	if err := s.Disable(context.Background()); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if fileExists(filepath.Join(dir, schedule.ServiceUnit)) || fileExists(filepath.Join(dir, schedule.TimerUnit)) {
		t.Error("expected both unit files removed after Disable")
	}
	if !rr.calledWith("systemctl disable --now themis.timer") {
		t.Errorf("expected disable --now, got %v", rr.calls)
	}
	st, err := s.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Installed {
		t.Error("status should report not installed after Disable")
	}
}

func TestSystemdDisableSurvivesSystemctlFailure(t *testing.T) {
	dir := t.TempDir()
	// systemctl disable fails (timer never enabled) — files must still go.
	rr := &recordingRunner{failOn: "disable", failWith: context.DeadlineExceeded}
	s := systemdScheduler{unitDir: dir, run: rr.run}
	if err := s.Enable(context.Background(), dailyCheckSpec("/x/themis")); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := s.Disable(context.Background()); err != nil {
		t.Fatalf("Disable should tolerate a failed systemctl disable: %v", err)
	}
	if fileExists(filepath.Join(dir, schedule.TimerUnit)) {
		t.Error("timer file must be removed even when systemctl disable fails")
	}
}

func TestSystemdEnableRejectsBadCommand(t *testing.T) {
	dir := t.TempDir()
	s := systemdScheduler{unitDir: dir, run: (&recordingRunner{}).run}
	err := s.Enable(context.Background(), schedule.Spec{BinaryPath: "/x/themis", Command: "rollback", Interval: "daily"})
	if err == nil {
		t.Fatal("expected error for unsupported command")
	}
	if fileExists(filepath.Join(dir, schedule.ServiceUnit)) {
		t.Error("no unit file should be written when the command is invalid")
	}
}

func TestCronEnableDisableStatus(t *testing.T) {
	dir := t.TempDir()
	c := cronScheduler{cronDir: dir}
	if err := c.Enable(context.Background(), dailyCheckSpec("/usr/local/bin/themis")); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, schedule.CronFile))
	if !strings.Contains(string(body), "0 0 * * * root /usr/local/bin/themis check") {
		t.Errorf("cron content wrong:\n%s", body)
	}
	st, err := c.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Installed || !st.Active {
		t.Errorf("status = %+v, want installed", st)
	}
	if err := c.Disable(context.Background()); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if fileExists(filepath.Join(dir, schedule.CronFile)) {
		t.Error("cron file should be removed")
	}
}

func TestCronEnableRejectsRawOnCalendar(t *testing.T) {
	dir := t.TempDir()
	c := cronScheduler{cronDir: dir}
	err := c.Enable(context.Background(), schedule.Spec{BinaryPath: "/x/themis", Command: "check", Interval: "*-*-* 03:00:00"})
	if err == nil {
		t.Fatal("cron must reject a raw systemd OnCalendar expression")
	}
}

func TestLaunchdEnableDisable(t *testing.T) {
	dir := t.TempDir()
	rr := &recordingRunner{}
	l := launchdScheduler{daemonDir: dir, run: rr.run}
	if err := l.Enable(context.Background(), schedule.Spec{BinaryPath: "/usr/local/bin/themis", Command: "check", Interval: "weekly"}); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	plist := filepath.Join(dir, schedule.LaunchdLabel+".plist")
	if !fileExists(plist) {
		t.Fatal("expected plist written")
	}
	if !rr.calledWith("launchctl bootstrap system") {
		t.Errorf("expected launchctl bootstrap, got %v", rr.calls)
	}
	if err := l.Disable(context.Background()); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if fileExists(plist) {
		t.Error("plist should be removed after Disable")
	}
	if !rr.calledWith("launchctl bootout system") {
		t.Errorf("expected launchctl bootout, got %v", rr.calls)
	}
}

func TestPrintScheduleStatus(t *testing.T) {
	var installed bytes.Buffer
	printScheduleStatus(&installed, scheduleStatus{Backend: "systemd", Installed: true, Active: true, Detail: "next in 3h"})
	if !strings.Contains(installed.String(), "active") || !strings.Contains(installed.String(), "systemd") {
		t.Errorf("installed status output = %q", installed.String())
	}

	var absent bytes.Buffer
	printScheduleStatus(&absent, scheduleStatus{Backend: "cron", Installed: false})
	if !strings.Contains(absent.String(), "no themis scheduled scan") {
		t.Errorf("absent status output = %q", absent.String())
	}
	if !strings.Contains(absent.String(), "schedule enable") {
		t.Errorf("absent status should hint at enable: %q", absent.String())
	}
}

func TestScheduleCmdRegistersSubcommands(t *testing.T) {
	want := map[string]bool{"enable": false, "disable": false, "status": false}
	for _, c := range scheduleCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected schedule subcommand %q to be registered", name)
		}
	}
}

func TestDetectSchedulerFallback(t *testing.T) {
	// A root with no run/systemd/system present: on linux this must fall
	// back to cron; on darwin it is always launchd. Either way detection
	// must succeed and never return systemd for a tree with no systemd.
	sched, err := detectScheduler(t.TempDir(), (&recordingRunner{}).run)
	if err != nil {
		t.Fatalf("detectScheduler: %v", err)
	}
	if sched.Backend() == "systemd" {
		t.Errorf("no run/systemd/system present but got systemd backend")
	}
}

// TestNormalizeRootEmptyIsAbsolute is the regression guard for the review
// finding: an empty root must resolve to "/", so every unit/probe path
// filepath.Join builds off it is absolute (leading slash) rather than a
// cwd-relative path that never matches /run/systemd/system on a real host.
func TestNormalizeRootEmptyIsAbsolute(t *testing.T) {
	root := normalizeRoot("")
	if root != "/" {
		t.Fatalf("normalizeRoot(\"\") = %q, want \"/\"", root)
	}
	for _, rel := range []string{"run/systemd/system", "etc/systemd/system", "etc/cron.d", "Library/LaunchDaemons"} {
		got := filepath.Join(root, rel)
		if !filepath.IsAbs(got) {
			t.Errorf("filepath.Join(normalizeRoot(\"\"), %q) = %q, want an absolute path", rel, got)
		}
		if want := "/" + rel; got != want {
			t.Errorf("filepath.Join(normalizeRoot(\"\"), %q) = %q, want %q", rel, got, want)
		}
	}
	// A non-empty root must pass through untouched so tests can still point
	// detection at a temp dir.
	if got := normalizeRoot("/tmp/x"); got != "/tmp/x" {
		t.Errorf("normalizeRoot(\"/tmp/x\") = %q, want it unchanged", got)
	}
}

// TestDetectSchedulerEmptyRootProducesAbsoluteDirs proves the production
// call site — detectScheduler("", ...) — hands each backend an absolute
// unit directory, whichever OS the test runs on.
func TestDetectSchedulerEmptyRootProducesAbsoluteDirs(t *testing.T) {
	sched, err := detectScheduler("", (&recordingRunner{}).run)
	if err != nil {
		t.Fatalf("detectScheduler: %v", err)
	}
	var dir, want string
	switch s := sched.(type) {
	case systemdScheduler:
		dir, want = s.unitDir, "/etc/systemd/system"
	case cronScheduler:
		dir, want = s.cronDir, "/etc/cron.d"
	case launchdScheduler:
		dir, want = s.daemonDir, "/Library/LaunchDaemons"
	default:
		t.Fatalf("unexpected scheduler type %T", sched)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("%s backend dir %q is not absolute", sched.Backend(), dir)
	}
	if dir != want {
		t.Errorf("%s backend dir = %q, want %q", sched.Backend(), dir, want)
	}
}
