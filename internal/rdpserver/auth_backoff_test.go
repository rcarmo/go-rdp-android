package rdpserver

import (
	"testing"
	"time"
)

func TestAuthBackoffLimiterProgressionAndReset(t *testing.T) {
	policy, err := normalizeAccessPolicy(AccessPolicy{FailedAuthLimit: 2, FailedAuthBackoff: 2 * time.Second, FailedAuthBackoffMax: 8 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	l := newAuthBackoffLimiter(policy)
	now := time.Unix(1_700_000_000, 0)
	l.now = func() time.Time { return now }

	if wait := l.lockoutRemaining("10.0.0.1", "rui"); wait != 0 {
		t.Fatalf("unexpected initial lockout: %v", wait)
	}
	if wait := l.recordFailure("10.0.0.1", "rui"); wait != 0 {
		t.Fatalf("unexpected first failure lockout: %v", wait)
	}
	if wait := l.recordFailure("10.0.0.1", "rui"); wait != 2*time.Second {
		t.Fatalf("unexpected second failure lockout: %v", wait)
	}
	if wait := l.lockoutRemaining("10.0.0.1", "rui"); wait != 2*time.Second {
		t.Fatalf("expected lockout remaining, got %v", wait)
	}

	now = now.Add(2 * time.Second)
	if wait := l.recordFailure("10.0.0.1", "rui"); wait != 4*time.Second {
		t.Fatalf("unexpected third failure lockout: %v", wait)
	}
	now = now.Add(4 * time.Second)
	if wait := l.recordFailure("10.0.0.1", "rui"); wait != 8*time.Second {
		t.Fatalf("unexpected fourth failure lockout cap: %v", wait)
	}

	l.recordSuccess("10.0.0.1", "rui")
	if wait := l.lockoutRemaining("10.0.0.1", "rui"); wait != 0 {
		t.Fatalf("expected reset after success, got %v", wait)
	}
}

func TestAuthBackoffLimiterSeparatesIdentities(t *testing.T) {
	policy, err := normalizeAccessPolicy(AccessPolicy{FailedAuthLimit: 1, FailedAuthBackoff: time.Second, FailedAuthBackoffMax: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	l := newAuthBackoffLimiter(policy)
	now := time.Unix(1_700_000_000, 0)
	l.now = func() time.Time { return now }

	if wait := l.recordFailure("10.0.0.1", "rui"); wait != time.Second {
		t.Fatalf("expected lockout for identity A, got %v", wait)
	}
	if wait := l.lockoutRemaining("10.0.0.1", "other"); wait != 0 {
		t.Fatalf("unexpected lockout bleed by username: %v", wait)
	}
	if wait := l.lockoutRemaining("10.0.0.2", "rui"); wait != 0 {
		t.Fatalf("unexpected lockout bleed by remote host: %v", wait)
	}
}
