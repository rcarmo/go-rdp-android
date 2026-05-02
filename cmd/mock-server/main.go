package main

import (
	"context"
	"flag"
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
	password := flag.String("password", "", "required password for Client Info authentication")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *addr, *width, *height, *testPattern, *fps, *username, *password); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string, width, height int, testPattern bool, fps int, username, password string) error {
	var frames frame.Source
	if testPattern {
		frames = frame.NewTestPatternSource(width, height, fps)
		defer frames.Close()
	}
	var auth rdpserver.Authenticator
	if username != "" || password != "" {
		auth = rdpserver.StaticCredentials{Username: username, Password: password}
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
