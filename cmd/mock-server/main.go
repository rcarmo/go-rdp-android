package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	"github.com/rcarmo/go-rdp-android/internal/rdpserver"
)

func main() {
	addr := flag.String("addr", ":3390", "listen address")
	width := flag.Int("width", 1280, "desktop width")
	height := flag.Int("height", 720, "desktop height")
	testPattern := flag.Bool("test-pattern", false, "feed animated synthetic frames")
	fps := flag.Int("fps", 5, "test pattern frame rate")
	username := flag.String("username", "", "required username for Client Info authentication")
	password := flag.String("password", "", "required plaintext password for Client Info/NLA authentication")
	passwordHash := flag.String("password-hash", "", "bcrypt password hash for Client Info authentication (TLS-only; NLA still needs plaintext password)")
	securityMode := flag.String("security-mode", string(rdpserver.SecurityModeNegotiate), "security policy mode: negotiate|rdp-only|tls-only|nla-required")
	allowedUsers := flag.String("allowed-users", "", "optional comma-separated allowed usernames")
	allowedCIDRs := flag.String("allowed-cidrs", "", "optional comma-separated client CIDR allowlist")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *addr, *width, *height, *testPattern, *fps, *username, *password, *passwordHash, *securityMode, splitCSV(*allowedUsers), splitCSV(*allowedCIDRs)); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string, width, height int, testPattern bool, fps int, username, password, passwordHash, securityMode string, allowedUsers, allowedCIDRs []string) error {
	var frames frame.Source
	if testPattern {
		frames = frame.NewTestPatternSource(width, height, fps)
		defer frames.Close()
	}
	var auth rdpserver.Authenticator
	if password != "" && passwordHash != "" {
		return fmt.Errorf("use either -password or -password-hash, not both")
	}
	if username != "" || password != "" {
		auth = rdpserver.StaticCredentials{Username: username, Password: password}
	}
	if passwordHash != "" {
		if username == "" {
			return fmt.Errorf("-username is required when using -password-hash")
		}
		auth = rdpserver.HashedCredentials{Username: username, PasswordHash: passwordHash}
	}
	srv, err := rdpserver.New(rdpserver.Config{
		Addr:          addr,
		Width:         width,
		Height:        height,
		Authenticator: auth,
		Policy: rdpserver.AccessPolicy{
			SecurityMode: rdpserver.SecurityMode(securityMode),
			AllowedUsers: allowedUsers,
			AllowedCIDRs: allowedCIDRs,
		},
	}, frames, nil)
	if err != nil {
		return err
	}
	log.Printf("listening on %s (protocol stub, testPattern=%v auth=%v security_mode=%s allowed_users=%d allowed_cidrs=%d)", addr, testPattern, auth != nil, securityMode, len(allowedUsers), len(allowedCIDRs))
	if err := srv.Listen(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := strings.TrimSpace(part)
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}
