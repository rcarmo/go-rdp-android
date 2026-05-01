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
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *addr, *width, *height, *testPattern, *fps); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string, width, height int, testPattern bool, fps int) error {
	var frames frame.Source
	if testPattern {
		frames = frame.NewTestPatternSource(width, height, fps)
		defer frames.Close()
	}
	srv, err := rdpserver.New(rdpserver.Config{Addr: addr, Width: width, Height: height}, frames, nil)
	if err != nil {
		return err
	}
	log.Printf("listening on %s (protocol stub, testPattern=%v)", addr, testPattern)
	if err := srv.Listen(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
