package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileH264Source(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frame.h264")
	data := []byte{0, 0, 0, 1, 0x65}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source, err := newFileH264Source(ctx, path, 30)
	if err != nil {
		t.Fatalf("newFileH264Source: %v", err)
	}
	select {
	case frame := <-source.H264Frames():
		if !frame.KeyFrame {
			t.Fatal("frame KeyFrame = false, want true")
		}
		if string(frame.Data) != string(data) {
			t.Fatalf("frame data = %x, want %x", frame.Data, data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for H.264 frame")
	}
}

func TestNewFileH264SourceRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.h264")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newFileH264Source(context.Background(), path, 1); err == nil {
		t.Fatal("newFileH264Source empty file error = nil, want error")
	}
}
