package rdpserver

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// SecurityMode controls which RDP security protocols the server accepts.
type SecurityMode string

const (
	SecurityModeNegotiate   SecurityMode = "negotiate"
	SecurityModeRDPOnly     SecurityMode = "rdp-only"
	SecurityModeTLSOnly     SecurityMode = "tls-only"
	SecurityModeNLARequired SecurityMode = "nla-required"
)

// AccessPolicy controls protocol selection and client access constraints.
type AccessPolicy struct {
	SecurityMode SecurityMode
	AllowedUsers []string
	AllowedCIDRs []string

	FailedAuthLimit      int
	FailedAuthBackoff    time.Duration
	FailedAuthBackoffMax time.Duration

	allowedUsers map[string]struct{}
	allowedCIDRs []*net.IPNet
}

func normalizeAccessPolicy(policy AccessPolicy) (AccessPolicy, error) {
	if policy.SecurityMode == "" {
		policy.SecurityMode = SecurityModeNegotiate
	}
	switch policy.SecurityMode {
	case SecurityModeNegotiate, SecurityModeRDPOnly, SecurityModeTLSOnly, SecurityModeNLARequired:
	default:
		return policy, fmt.Errorf("unsupported security mode %q", policy.SecurityMode)
	}
	if policy.FailedAuthLimit < 0 {
		return policy, fmt.Errorf("failed auth limit must be >= 0")
	}
	if policy.FailedAuthBackoff < 0 {
		return policy, fmt.Errorf("failed auth backoff must be >= 0")
	}
	if policy.FailedAuthBackoffMax < 0 {
		return policy, fmt.Errorf("failed auth backoff max must be >= 0")
	}
	if policy.FailedAuthLimit > 0 {
		if policy.FailedAuthBackoff == 0 {
			policy.FailedAuthBackoff = 2 * time.Second
		}
		if policy.FailedAuthBackoffMax == 0 {
			policy.FailedAuthBackoffMax = time.Minute
		}
		if policy.FailedAuthBackoffMax < policy.FailedAuthBackoff {
			policy.FailedAuthBackoffMax = policy.FailedAuthBackoff
		}
	}
	if len(policy.AllowedUsers) > 0 {
		policy.allowedUsers = make(map[string]struct{}, len(policy.AllowedUsers))
		for _, user := range policy.AllowedUsers {
			normalized := strings.TrimSpace(user)
			if normalized == "" {
				continue
			}
			policy.allowedUsers[normalized] = struct{}{}
		}
	}
	if len(policy.AllowedCIDRs) > 0 {
		policy.allowedCIDRs = make([]*net.IPNet, 0, len(policy.AllowedCIDRs))
		for _, cidr := range policy.AllowedCIDRs {
			normalized := strings.TrimSpace(cidr)
			if normalized == "" {
				continue
			}
			_, network, err := net.ParseCIDR(normalized)
			if err != nil {
				return policy, fmt.Errorf("invalid allowed CIDR %q: %w", normalized, err)
			}
			policy.allowedCIDRs = append(policy.allowedCIDRs, network)
		}
	}
	return policy, nil
}

func (p AccessPolicy) protocol(requested uint32) (uint32, error) {
	return selectNegotiatedProtocolWithMode(requested, p.SecurityMode)
}

func (p AccessPolicy) userAllowed(username string) bool {
	if len(p.allowedUsers) == 0 {
		return true
	}
	_, ok := p.allowedUsers[strings.TrimSpace(username)]
	return ok
}

func (p AccessPolicy) remoteAllowed(addr net.Addr) bool {
	if len(p.allowedCIDRs) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range p.allowedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
