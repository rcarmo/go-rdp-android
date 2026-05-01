package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/rcarmo/go-rdp-android/internal/rdpserver"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, ":3390"); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string) error {
	srv, err := rdpserver.New(rdpserver.Config{Addr: addr, Width: 1280, Height: 720}, nil, nil)
	if err != nil {
		return err
	}
	log.Printf("listening on %s (protocol stub)", addr)
	if err := srv.Listen(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
