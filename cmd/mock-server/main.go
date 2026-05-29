package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	failedAuthLimit := flag.Int("failed-auth-limit", 0, "failed auth attempts before lockout (0 disables lockout/backoff)")
	failedAuthBackoff := flag.Duration("failed-auth-backoff", 2*time.Second, "initial lockout/backoff duration once failed-auth-limit is hit")
	failedAuthBackoffMax := flag.Duration("failed-auth-backoff-max", time.Minute, "maximum lockout/backoff duration")
	tlsCert := flag.String("tls-cert", "", "optional TLS certificate PEM path (persisted/loaded when paired with -tls-key)")
	tlsKey := flag.String("tls-key", "", "optional TLS private key PEM path (persisted/loaded when paired with -tls-cert)")
	tlsRotate := flag.Bool("tls-rotate", false, "rotate/regenerate TLS certificate at startup when using -tls-cert/-tls-key")
	tlsCN := flag.String("tls-cn", "go-rdp-android", "TLS certificate common name when generating a self-signed cert")
	h264File := flag.String("h264-file", "", "optional Annex B or length-prefixed H.264 access-unit file to feed the experimental RDPGFX AVC420 path")
	h264FPS := flag.Int("h264-fps", 5, "frame rate for replaying -h264-file access units")
	rfxFile := flag.String("rfx-file", "", "optional encoded RemoteFX fixture payload for experimental SurfaceBits transport")
	clearCodecFile := flag.String("clearcodec-file", "", "optional encoded ClearCodec fixture payload for experimental RDPGFX transport")
	progressiveFile := flag.String("progressive-file", "", "optional encoded Progressive fixture payload for experimental RDPGFX transport")
	progressiveV2File := flag.String("progressivev2-file", "", "optional encoded ProgressiveV2 fixture payload for experimental RDPGFX transport")
	avc444File := flag.String("avc444-file", "", "optional encoded AVC444 fixture payload for experimental RDPGFX transport")
	avc444v2File := flag.String("avc444v2-file", "", "optional encoded AVC444v2 fixture payload for experimental RDPGFX transport")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *addr, *width, *height, *testPattern, *fps, *h264File, *h264FPS, *username, *password, *passwordHash, *securityMode, splitCSV(*allowedUsers), splitCSV(*allowedCIDRs), *failedAuthLimit, *failedAuthBackoff, *failedAuthBackoffMax, *rfxFile, *clearCodecFile, *progressiveFile, *progressiveV2File, *avc444File, *avc444v2File, rdpserver.TLSSettings{CertFile: *tlsCert, KeyFile: *tlsKey, RotateOnStart: *tlsRotate, CommonName: *tlsCN}); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string, width, height int, testPattern bool, fps int, h264File string, h264FPS int, username, password, passwordHash, securityMode string, allowedUsers, allowedCIDRs []string, failedAuthLimit int, failedAuthBackoff, failedAuthBackoffMax time.Duration, rfxFile, clearCodecFile, progressiveFile, progressiveV2File, avc444File, avc444v2File string, tlsSettings rdpserver.TLSSettings) error {
	var frames frame.Source
	if testPattern {
		frames = frame.NewTestPatternSource(width, height, fps)
		defer frames.Close()
	}
	var h264 rdpserver.H264Source
	if strings.TrimSpace(h264File) != "" {
		source, err := newFileH264Source(ctx, h264File, h264FPS)
		if err != nil {
			return err
		}
		h264 = source
	}

	rfxEncoder, err := newFixtureFrameEncoder(rfxFile)
	if err != nil {
		return fmt.Errorf("load RFX fixture: %w", err)
	}
	clearCodecEncoder, err := newFixtureFrameEncoder(clearCodecFile)
	if err != nil {
		return fmt.Errorf("load ClearCodec fixture: %w", err)
	}
	progressiveEncoder, err := newFixtureFrameEncoder(progressiveFile)
	if err != nil {
		return fmt.Errorf("load Progressive fixture: %w", err)
	}
	progressiveV2Encoder, err := newFixtureFrameEncoder(progressiveV2File)
	if err != nil {
		return fmt.Errorf("load ProgressiveV2 fixture: %w", err)
	}
	avc444Encoder, err := newFixtureFrameEncoder(avc444File)
	if err != nil {
		return fmt.Errorf("load AVC444 fixture: %w", err)
	}
	avc444v2Encoder, err := newFixtureFrameEncoder(avc444v2File)
	if err != nil {
		return fmt.Errorf("load AVC444v2 fixture: %w", err)
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
			SecurityMode:         rdpserver.SecurityMode(securityMode),
			AllowedUsers:         allowedUsers,
			AllowedCIDRs:         allowedCIDRs,
			FailedAuthLimit:      failedAuthLimit,
			FailedAuthBackoff:    failedAuthBackoff,
			FailedAuthBackoffMax: failedAuthBackoffMax,
		},
		TLS:           tlsSettings,
		H264:          h264,
		RFX:           rfxEncoder,
		ClearCodec:    clearCodecEncoder,
		Progressive:   progressiveEncoder,
		ProgressiveV2: progressiveV2Encoder,
		AVC444:        avc444Encoder,
		AVC444v2:      avc444v2Encoder,
	}, frames, nil)
	if err != nil {
		return err
	}
	log.Printf("listening on %s (protocol stub, testPattern=%v h264File=%v auth=%v security_mode=%s allowed_users=%d allowed_cidrs=%d failed_auth_limit=%d rfx_fixture=%t clearcodec_fixture=%t progressive_fixture=%t progressivev2_fixture=%t avc444_fixture=%t avc444v2_fixture=%t tls_fp=%s)", addr, testPattern, h264 != nil, auth != nil, securityMode, len(allowedUsers), len(allowedCIDRs), failedAuthLimit, rfxEncoder != nil, clearCodecEncoder != nil, progressiveEncoder != nil, progressiveV2Encoder != nil, avc444Encoder != nil, avc444v2Encoder != nil, srv.TLSFingerprintSHA256())
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
