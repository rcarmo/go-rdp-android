package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestNewFixtureFrameEncoder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codec.bin")
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	enc, err := newFixtureFrameEncoder(path)
	if err != nil {
		t.Fatalf("newFixtureFrameEncoder: %v", err)
	}
	if enc == nil {
		t.Fatal("encoder = nil")
	}
	rfx, ok := enc.EncodeRFX(frame.Frame{}, 1, 1)
	if !ok || string(rfx) != string([]byte{1, 2, 3}) {
		t.Fatalf("EncodeRFX = %x,%t", rfx, ok)
	}
	rdpgfx, ok := enc.EncodeRDPGFX(frame.Frame{}, 1, 1)
	if !ok || string(rdpgfx) != string([]byte{1, 2, 3}) {
		t.Fatalf("EncodeRDPGFX = %x,%t", rdpgfx, ok)
	}
	rfx[0] = 9
	again, _ := enc.EncodeRFX(frame.Frame{}, 1, 1)
	if again[0] != 1 {
		t.Fatalf("encoder returned mutable backing store: %x", again)
	}
}

func TestNewFixtureFrameEncoderRejectsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.bin")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if enc, err := newFixtureFrameEncoder(path); err == nil || enc != nil {
		t.Fatalf("newFixtureFrameEncoder empty = %#v,%v", enc, err)
	}
}
