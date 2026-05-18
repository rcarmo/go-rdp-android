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

func TestSplitAnnexBH264AccessUnits(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0x67, 1, 0, 0, 1, 0x65, 2}
	units := splitAnnexBH264AccessUnits(data)
	if len(units) != 2 {
		t.Fatalf("len(units) = %d, want 2", len(units))
	}
	if string(units[0]) != string([]byte{0, 0, 0, 1, 0x67, 1}) {
		t.Fatalf("unit[0] = %x", units[0])
	}
	if string(units[1]) != string([]byte{0, 0, 1, 0x65, 2}) {
		t.Fatalf("unit[1] = %x", units[1])
	}
	if !h264FixtureContainsIDR(units[1]) {
		t.Fatal("unit[1] should contain IDR")
	}
	if h264FixtureContainsIDR(units[0]) {
		t.Fatal("unit[0] should not contain IDR")
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
