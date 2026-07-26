package schedule

import (
	"strings"
	"testing"
)

func TestExecArgs(t *testing.T) {
	tests := []struct {
		command string
		want    string
		wantErr bool
	}{
		{"check", "check", false},
		{"apply", "apply --yes", false},
		{"rollback", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := ExecArgs(tt.command)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ExecArgs(%q): want error, got %v", tt.command, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ExecArgs(%q): %v", tt.command, err)
			continue
		}
		if strings.Join(got, " ") != tt.want {
			t.Errorf("ExecArgs(%q) = %v, want %q", tt.command, got, tt.want)
		}
	}
}

func TestSystemdOnCalendar(t *testing.T) {
	for _, tt := range []struct {
		in, want string
		wantErr  bool
	}{
		{"daily", "daily", false},
		{"weekly", "weekly", false},
		{"*-*-* 03:00:00", "*-*-* 03:00:00", false},
		{"", "", true},
		{"   ", "", true},
	} {
		got, err := SystemdOnCalendar(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("SystemdOnCalendar(%q): want error", tt.in)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("SystemdOnCalendar(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestLaunchdCalendarFor(t *testing.T) {
	daily, err := LaunchdCalendarFor("daily")
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if daily.HasWeekday || daily.Hour != 0 || daily.Minute != 0 {
		t.Errorf("daily = %+v, want midnight no weekday", daily)
	}
	weekly, err := LaunchdCalendarFor("weekly")
	if err != nil {
		t.Fatalf("weekly: %v", err)
	}
	if !weekly.HasWeekday || weekly.Weekday != 0 {
		t.Errorf("weekly = %+v, want Sunday", weekly)
	}
	if _, err := LaunchdCalendarFor("*-*-* 03:00:00"); err == nil {
		t.Error("raw OnCalendar: want error on launchd")
	}
}

func TestCronExprFor(t *testing.T) {
	for _, tt := range []struct {
		in, want string
		wantErr  bool
	}{
		{"daily", "0 0 * * *", false},
		{"weekly", "0 0 * * 0", false},
		{"*-*-* 03:00:00", "", true},
	} {
		got, err := CronExprFor(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("CronExprFor(%q): want error", tt.in)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("CronExprFor(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestSystemdServiceCheck(t *testing.T) {
	got, err := SystemdService(Spec{BinaryPath: "/usr/local/bin/themis", Command: "check", Interval: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Type=oneshot") {
		t.Errorf("service missing Type=oneshot:\n%s", got)
	}
	if !strings.Contains(got, "ExecStart=/usr/local/bin/themis check\n") {
		t.Errorf("service ExecStart wrong:\n%s", got)
	}
}

func TestSystemdServiceApplyIsNonInteractive(t *testing.T) {
	got, err := SystemdService(Spec{BinaryPath: "/usr/local/bin/themis", Command: "apply", Interval: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "ExecStart=/usr/local/bin/themis apply --yes\n") {
		t.Errorf("apply ExecStart must pass --yes for unattended runs:\n%s", got)
	}
}

func TestSystemdServiceRejectsBadCommand(t *testing.T) {
	if _, err := SystemdService(Spec{BinaryPath: "/x/themis", Command: "nope"}); err == nil {
		t.Error("want error for unsupported command")
	}
}

func TestSystemdTimer(t *testing.T) {
	got, err := SystemdTimer(Spec{BinaryPath: "/x/themis", Command: "check", Interval: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OnCalendar=daily", "Persistent=true", "Unit=" + ServiceUnit, "WantedBy=timers.target"} {
		if !strings.Contains(got, want) {
			t.Errorf("timer missing %q:\n%s", want, got)
		}
	}
}

func TestSystemdTimerRejectsEmptyInterval(t *testing.T) {
	if _, err := SystemdTimer(Spec{BinaryPath: "/x/themis", Command: "check", Interval: ""}); err == nil {
		t.Error("want error for empty interval")
	}
}

func TestLaunchdPlistWeeklyApply(t *testing.T) {
	got, err := LaunchdPlist(Spec{BinaryPath: "/usr/local/bin/themis", Command: "apply", Interval: "weekly"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<string>" + LaunchdLabel + "</string>",
		"<string>/usr/local/bin/themis</string>",
		"<string>apply</string>",
		"<string>--yes</string>",
		"<key>Weekday</key>",
		"<key>Hour</key>",
		"<key>RunAtLoad</key>\n\t<false/>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q:\n%s", want, got)
		}
	}
}

func TestLaunchdPlistDailyHasNoWeekday(t *testing.T) {
	got, err := LaunchdPlist(Spec{BinaryPath: "/x/themis", Command: "check", Interval: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<key>Weekday</key>") {
		t.Errorf("daily plist must not pin a weekday:\n%s", got)
	}
}

func TestCronContent(t *testing.T) {
	got, err := CronContent(Spec{BinaryPath: "/usr/local/bin/themis", Command: "check", Interval: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "0 0 * * * root /usr/local/bin/themis check\n") {
		t.Errorf("cron content wrong:\n%s", got)
	}
}
