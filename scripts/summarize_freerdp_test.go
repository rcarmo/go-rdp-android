package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummarizeFreeRDPRFXWriteFields(t *testing.T) {
	dir := runSummarizerForTest(t, "trace phase=rfx_codec_write codec_id=4 bytes=40 raw_bytes=100 saved_bytes=60\n")
	data, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatalf("decode summary.json: %v", err)
	}
	for key, want := range map[string]any{
		"rfx_codec_write_seen":    true,
		"rfx_codec_write_count":   float64(1),
		"rfx_codec_write_bytes":   float64(40),
		"rfx_codec_raw_bytes":     float64(100),
		"rfx_codec_saved_bytes":   float64(60),
		"rfx_codec_saved_percent": float64(60),
	} {
		if got := summary[key]; got != want {
			t.Fatalf("summary[%s] = %v, want %v", key, got, want)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "summary.md"))
	if err != nil {
		t.Fatalf("read summary.md: %v", err)
	}
	for _, want := range []string{
		"- RemoteFX write trace seen: `true`",
		"- RemoteFX write trace count: `1`",
		"- RemoteFX write trace bytes: `40`",
		"- RemoteFX raw bytes: `100`",
		"- RemoteFX saved bytes: `60`",
		"- RemoteFX saved percent: `60.0`",
	} {
		if !strings.Contains(string(markdown), want) {
			t.Fatalf("summary.md does not contain %q\n%s", want, markdown)
		}
	}
}

func TestSummarizeFreeRDPRDPGFXEncoderHookWriteCounts(t *testing.T) {
	dir := runSummarizerForTest(t, "trace phase=rdpgfx_frame_write frame_id=1 path=rdpgfx-clearcodec pdus=3 bytes=10\n"+
		"trace phase=rdpgfx_frame_write frame_id=2 path=rdpgfx-progressive pdus=3 bytes=11\n"+
		"trace phase=rdpgfx_frame_write frame_id=3 path=rdpgfx-avc444 pdus=3 bytes=12\n"+
		"trace phase=rdpgfx_frame_write frame_id=4 path=rdpgfx-avc444v2 pdus=3 bytes=13\n")
	data, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatalf("decode summary.json: %v", err)
	}
	for key, want := range map[string]float64{
		"rdpgfx_frame_write_count":       4,
		"rdpgfx_frame_write_bytes":       46,
		"rdpgfx_clearcodec_write_count":  1,
		"rdpgfx_progressive_write_count": 1,
		"rdpgfx_avc444_write_count":      1,
		"rdpgfx_avc444v2_write_count":    1,
	} {
		got, ok := summary[key].(float64)
		if !ok || got != want {
			t.Fatalf("summary[%s] = %v, want %v", key, summary[key], want)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "summary.md"))
	if err != nil {
		t.Fatalf("read summary.md: %v", err)
	}
	for _, want := range []string{
		"- RDPGFX ClearCodec write trace count: `1`",
		"- RDPGFX Progressive write trace count: `1`",
		"- RDPGFX AVC444 write trace count: `1`",
		"- RDPGFX AVC444v2 write trace count: `1`",
	} {
		if !strings.Contains(string(markdown), want) {
			t.Fatalf("summary.md does not contain %q\n%s", want, markdown)
		}
	}
}

func TestSummarizeFreeRDPDeferredCodecFields(t *testing.T) {
	dir := runSummarizerForTest(t, "trace phase=rfx_codec_selected codec_id=4 emission=deferred reason=encoder-missing\n"+
		"trace phase=rdpgfx_clearcodec_selected version=0x00080105 flags=0x00000000 codec_id=0x0008 emission=deferred reason=encoder-missing\n"+
		"trace phase=rdpgfx_progressive_selected version=0x000a0002 flags=0x00000000 codec_id=0x0009 codec_id_v2=0x000d emission=deferred reason=encoder-missing\n"+
		"trace phase=rdpgfx_avc444_selected version=0x000a0002 flags=0x00000000 codec_id=0x000e emission=deferred reason=transport-missing\n"+
		"trace phase=rdpgfx_avc444v2_selected version=0x000a0400 flags=0x00000000 codec_id=0x000f emission=deferred reason=transport-missing\n")
	data, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatalf("decode summary.json: %v", err)
	}
	for key, want := range map[string]any{
		"rfx_codec_selected":          true,
		"rfx_codec_reason":            "encoder-missing",
		"rdpgfx_clearcodec_selected":  true,
		"rdpgfx_clearcodec_reason":    "encoder-missing",
		"rdpgfx_progressive_selected": true,
		"rdpgfx_progressive_reason":   "encoder-missing",
		"rdpgfx_avc444_selected":      true,
		"rdpgfx_avc444_reason":        "transport-missing",
		"rdpgfx_avc444v2_selected":    true,
		"rdpgfx_avc444v2_reason":      "transport-missing",
	} {
		if got := summary[key]; got != want {
			t.Fatalf("summary[%s] = %v, want %v", key, got, want)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "summary.md"))
	if err != nil {
		t.Fatalf("read summary.md: %v", err)
	}
	for _, want := range []string{
		"- RemoteFX selected trace seen: `true`",
		"- RemoteFX selected reason: `encoder-missing`",
		"- RDPGFX ClearCodec selected trace seen: `true`",
		"- RDPGFX ClearCodec selected reason: `encoder-missing`",
		"- RDPGFX Progressive selected trace seen: `true`",
		"- RDPGFX Progressive selected reason: `encoder-missing`",
		"- RDPGFX AVC444 selected trace seen: `true`",
		"- RDPGFX AVC444 selected reason: `transport-missing`",
		"- RDPGFX AVC444v2 selected trace seen: `true`",
		"- RDPGFX AVC444v2 selected reason: `transport-missing`",
	} {
		if !strings.Contains(string(markdown), want) {
			t.Fatalf("summary.md does not contain %q\n%s", want, markdown)
		}
	}
}

func TestSummarizeFreeRDPRawSavedPercentFields(t *testing.T) {
	dir := runSummarizerForTest(t, "trace phase=nscodec_write codec_id=1 bytes=40 raw_bytes=100 saved_bytes=60\n"+
		"trace phase=jpeg_codec_write codec_id=2 bytes=75 raw_bytes=300 saved_bytes=225\n"+
		"trace phase=png_codec_write codec_id=3 bytes=90 raw_bytes=120 saved_bytes=30\n")
	data, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatalf("decode summary.json: %v", err)
	}
	for key, want := range map[string]float64{
		"nscodec_raw_bytes":        100,
		"nscodec_saved_bytes":      60,
		"nscodec_saved_percent":    60,
		"jpeg_codec_raw_bytes":     300,
		"jpeg_codec_saved_bytes":   225,
		"jpeg_codec_saved_percent": 75,
		"png_codec_raw_bytes":      120,
		"png_codec_saved_bytes":    30,
		"png_codec_saved_percent":  25,
	} {
		got, ok := summary[key].(float64)
		if !ok || got != want {
			t.Fatalf("summary[%s] = %v, want %v", key, summary[key], want)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "summary.md"))
	if err != nil {
		t.Fatalf("read summary.md: %v", err)
	}
	for _, want := range []string{
		"- NSCodec raw bytes: `100`",
		"- NSCodec saved bytes: `60`",
		"- NSCodec saved percent: `60.0`",
		"- JPEG codec raw bytes: `300`",
		"- JPEG codec saved bytes: `225`",
		"- JPEG codec saved percent: `75.0`",
		"- PNG codec raw bytes: `120`",
		"- PNG codec saved bytes: `30`",
		"- PNG codec saved percent: `25.0`",
	} {
		if !strings.Contains(string(markdown), want) {
			t.Fatalf("summary.md does not contain %q\n%s", want, markdown)
		}
	}
}

func runSummarizerForTest(t *testing.T, serverLog string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mock-server.log"), []byte(serverLog), 0o644); err != nil {
		t.Fatalf("write mock-server.log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "xfreerdp.log"), []byte("CONNECTION_STATE_ACTIVE\n"), 0o644); err != nil {
		t.Fatalf("write xfreerdp.log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exit-code.txt"), []byte("0\n"), 0o644); err != nil {
		t.Fatalf("write exit-code.txt: %v", err)
	}
	goTmp := filepath.Join(dir, "gotmp")
	if err := os.MkdirAll(goTmp, 0o755); err != nil {
		t.Fatalf("mkdir GOTMPDIR: %v", err)
	}
	cmd := exec.Command("go", "run", "./summarize-freerdp.go", dir)
	cmd.Env = append(os.Environ(), "GOTMPDIR="+goTmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("summarize-freerdp.go failed: %v\n%s", err, out)
	}
	return dir
}
