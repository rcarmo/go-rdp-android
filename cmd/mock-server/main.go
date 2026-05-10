package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
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
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *addr, *width, *height, *testPattern, *fps, *username, *password, *passwordHash); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string, width, height int, testPattern bool, fps int, username, password, passwordHash string) error {
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
	srv, err := rdpserver.New(rdpserver.Config{Addr: addr, Width: width, Height: height, Authenticator: auth}, frames, nil)
	if err != nil {
		return err
	}
	log.Printf("listening on %s (protocol stub, testPattern=%v auth=%v)", addr, testPattern, auth != nil)
	if err := srv.Listen(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
