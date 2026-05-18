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
	if !h264FixtureContainsNALType(units[0], 7) {
		t.Fatal("unit[0] should contain SPS")
	}
}

func TestCoalesceH264FixtureConfigUnits(t *testing.T) {
	units := [][]byte{
		{0, 0, 0, 1, 0x67, 1},
		{0, 0, 0, 1, 0x68, 2},
		{0, 0, 0, 1, 0x65, 3},
	}
	coalesced := coalesceH264FixtureConfigUnits(units)
	if len(coalesced) != 2 {
		t.Fatalf("len(coalesced) = %d, want 2", len(coalesced))
	}
	wantConfig := append(append([]byte(nil), units[0]...), units[1]...)
	if string(coalesced[0]) != string(wantConfig) {
		t.Fatalf("coalesced config = %x, want %x", coalesced[0], wantConfig)
	}
	if string(coalesced[1]) != string(units[2]) {
		t.Fatalf("coalesced keyframe = %x, want %x", coalesced[1], units[2])
	}
}

func TestNewFileH264SourceMarksCodecConfigUnits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frames.h264")
	data := []byte{0, 0, 0, 1, 0x67, 1, 0, 0, 0, 1, 0x68, 2, 0, 0, 0, 1, 0x65, 3}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source, err := newFileH264Source(ctx, path, 60)
	if err != nil {
		t.Fatalf("newFileH264Source: %v", err)
	}
	first := <-source.H264Frames()
	second := <-source.H264Frames()
	if !first.CodecConfig || first.KeyFrame {
		t.Fatalf("first frame CodecConfig=%t KeyFrame=%t, want config-only", first.CodecConfig, first.KeyFrame)
	}
	if !h264FixtureContainsNALType(first.Data, 7) || !h264FixtureContainsNALType(first.Data, 8) {
		t.Fatalf("first frame data = %x, want SPS+PPS", first.Data)
	}
	if !second.KeyFrame || second.CodecConfig {
		t.Fatalf("second frame CodecConfig=%t KeyFrame=%t, want keyframe", second.CodecConfig, second.KeyFrame)
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
