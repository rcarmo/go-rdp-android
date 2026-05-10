package rdpserver

import (
	"net"
	"testing"
)

func TestNormalizeAccessPolicy(t *testing.T) {
	policy, err := normalizeAccessPolicy(AccessPolicy{AllowedUsers: []string{" rui "}, AllowedCIDRs: []string{"127.0.0.0/8"}})
	if err != nil {
		t.Fatal(err)
	}
	if policy.SecurityMode != SecurityModeNegotiate {
		t.Fatalf("unexpected default mode %q", policy.SecurityMode)
	}
	if !policy.userAllowed("rui") || policy.userAllowed("other") {
		t.Fatalf("unexpected allowed users behavior")
	}
	if !policy.remoteAllowed(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3390}) {
		t.Fatal("expected 127.0.0.1 to be allowed")
	}
	if policy.remoteAllowed(&net.TCPAddr{IP: net.ParseIP("10.0.0.2"), Port: 3390}) {
		t.Fatal("unexpected remote allow")
	}
}

func TestNormalizeAccessPolicyRejectsBadCIDR(t *testing.T) {
	if _, err := normalizeAccessPolicy(AccessPolicy{AllowedCIDRs: []string{"bad"}}); err == nil {
		t.Fatal("expected CIDR parse error")
	}
}
