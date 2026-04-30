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

	srv, err := rdpserver.New(rdpserver.Config{Addr: ":3390", Width: 1280, Height: 720}, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listening on :3390 (protocol stub)")
	if err := srv.Listen(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
