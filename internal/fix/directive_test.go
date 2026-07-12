package fix

import "testing"

func TestDirectiveValue(t *testing.T) {
	content := "Port 22\n#PermitRootLogin no\nPermitRootLogin yes\n"
	if got := directiveValue(content, "PermitRootLogin"); got != "yes" {
		t.Errorf("got %q, want %q", got, "yes")
	}
	if got := directiveValue(content, "PasswordAuthentication"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestDirectiveValueLastWins(t *testing.T) {
	content := "PermitRootLogin yes\nPermitRootLogin no\n"
	if got := directiveValue(content, "PermitRootLogin"); got != "no" {
		t.Errorf("got %q, want %q", got, "no")
	}
}

func TestSetDirectiveAppendsWhenMissing(t *testing.T) {
	got := setDirective("Port 22\n", "PermitRootLogin", "no")
	want := "Port 22\nPermitRootLogin no"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSetDirectiveNoopWhenAlreadyCorrect(t *testing.T) {
	content := "Port 22\nPermitRootLogin no\n"
	got := setDirective(content, "PermitRootLogin", "no")
	if directiveValue(got, "PermitRootLogin") != "no" {
		t.Errorf("expected PermitRootLogin no to remain effective, got %q", got)
	}
}

func TestSetDirectiveCommentsOutConflicting(t *testing.T) {
	content := "PermitRootLogin yes\n"
	got := setDirective(content, "PermitRootLogin", "no")
	if directiveValue(got, "PermitRootLogin") != "no" {
		t.Errorf("expected PermitRootLogin no to be effective, got %q", got)
	}
	if got == content {
		t.Errorf("expected content to change")
	}
}
