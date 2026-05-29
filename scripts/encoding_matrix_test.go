package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestEncodingMatrixIncludesRFXProductionAndFixtureCases(t *testing.T) {
	data, err := os.ReadFile("encoding-matrix.sh")
	if err != nil {
		t.Fatalf("read encoding-matrix.sh: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"run_case rfx-encoded",
		"run_case rfx-fixture",
		"-rfx-file $OUT/codec-fixture.bin",
		"rfx-fixture\", \"rdpgfx-planar",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("encoding-matrix.sh does not contain %q", want)
		}
	}
}

func TestEncodingMatrixIncludesRDPGFXFixtureCases(t *testing.T) {
	data, err := os.ReadFile("encoding-matrix.sh")
	if err != nil {
		t.Fatalf("read encoding-matrix.sh: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"run_case rdpgfx-clearcodec-encoded",
		"run_case rdpgfx-clearcodec-fixture",
		"-clearcodec-file $OUT/codec-fixture.bin",
		"run_case rdpgfx-progressive-encoded",
		"run_case rdpgfx-progressivev2-encoded",
		"rdpgfx_progressive_v2_selected",
		"run_case rdpgfx-progressive-fixture",
		"-progressive-file $OUT/codec-fixture.bin",
		"run_case rdpgfx-progressivev2-fixture",
		"-progressivev2-file $OUT/codec-fixture.bin",
		"run_case rdpgfx-avc444-encoded",
		"run_case rdpgfx-avc444-fixture",
		"-avc444-file $OUT/codec-fixture.bin",
		"run_case rdpgfx-avc444v2-encoded",
		"run_case rdpgfx-avc444v2-fixture",
		"-avc444v2-file $OUT/codec-fixture.bin",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("encoding-matrix.sh does not contain %q", want)
		}
	}
}

func TestEncodingMatrixIncludesClassicBitmapCases(t *testing.T) {
	data, err := os.ReadFile("encoding-matrix.sh")
	if err != nil {
		t.Fatalf("read encoding-matrix.sh: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"run_case bitmap-planar",
		"GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR=1",
		"bitmap_planar_seen",
		"Classic RDP6 bitmap-update Planar",
		"run_case bitmap-16bpp",
		"GO_RDP_ANDROID_ENABLE_BITMAP_BPP=16",
		"bitmap_bpp16_seen",
		"Classic 16bpp bitmap updates",
		"run_case bitmap-15bpp",
		"GO_RDP_ANDROID_ENABLE_BITMAP_BPP=15",
		"bitmap_bpp15_seen",
		"Classic 15bpp bitmap updates",
		"run_case bitmap-8bpp",
		"GO_RDP_ANDROID_ENABLE_BITMAP_BPP=8",
		"bitmap_bpp8_seen",
		"palette_seen",
		"Classic 8bpp paletted bitmap updates",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("encoding-matrix.sh does not contain %q", want)
		}
	}
}

func TestEncodingMatrixIncludesPNGOptInCase(t *testing.T) {
	data, err := os.ReadFile("encoding-matrix.sh")
	if err != nil {
		t.Fatalf("read encoding-matrix.sh: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"run_case png-opt-in",
		"GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID=9",
		"GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL=-3",
		"png-opt-in\", \"rfx-encoded",
		"PNG opt-in should at least reach active state",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("encoding-matrix.sh does not contain %q", want)
		}
	}
}

func TestEncodingMatrixCodecCoverageJSONShape(t *testing.T) {
	data, err := os.ReadFile("encoding-matrix.sh")
	if err != nil {
		t.Fatalf("read encoding-matrix.sh: %v", err)
	}
	jsonText := extractHereDoc(t, string(data), "cat >\"$OUT/codec-coverage.json\" <<'JSON'", "JSON")
	var coverage struct {
		RuntimeEmitters                []map[string]any `json:"runtime_emitters"`
		SelectionScaffolds             []map[string]any `json:"selection_scaffolds"`
		UpstreamMetadata               []map[string]any `json:"upstream_metadata"`
		MissingRuntimeEmitters         []map[string]any `json:"missing_runtime_emitters"`
		ReleaseDefaults                []string         `json:"release_defaults"`
		NonDefaultExperimentalEmitters []string         `json:"non_default_experimental_emitters"`
	}
	if err := json.Unmarshal([]byte(jsonText), &coverage); err != nil {
		t.Fatalf("decode codec coverage JSON: %v\n%s", err, jsonText)
	}
	if len(coverage.RuntimeEmitters) == 0 {
		t.Fatal("runtime_emitters is empty")
	}
	if len(coverage.UpstreamMetadata) == 0 {
		t.Fatal("upstream_metadata is empty")
	}
	if len(coverage.MissingRuntimeEmitters) != 0 {
		t.Fatalf("missing_runtime_emitters should be empty after production encoder coverage, got %#v", coverage.MissingRuntimeEmitters)
	}
	if len(coverage.ReleaseDefaults) == 0 {
		t.Fatal("release_defaults is empty")
	}
	if len(coverage.NonDefaultExperimentalEmitters) == 0 {
		t.Fatal("non_default_experimental_emitters is empty")
	}
	assertEntryNamed(t, coverage.RuntimeEmitters, "Classic RDP6 bitmap-update Planar")
	assertEntryNamed(t, coverage.RuntimeEmitters, "Classic 16bpp bitmap updates")
	assertEntryNamed(t, coverage.RuntimeEmitters, "Classic 15bpp bitmap updates")
	assertEntryNamed(t, coverage.RuntimeEmitters, "Classic 8bpp paletted bitmap updates")
	assertEntryNamed(t, coverage.RuntimeEmitters, "PNG bitmap codec")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX Planar")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RemoteFX / RFX")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX ClearCodec")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX Progressive / other progressive codecs")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX AVC444")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX AVC444v2")
	assertEntryHasBool(t, coverage.RuntimeEmitters, "RDPGFX Planar", "release_default", true)
	assertEntryHasBool(t, coverage.RuntimeEmitters, "RDPGFX ClearCodec", "fixture_hook", true)
	assertEntryHasBool(t, coverage.RuntimeEmitters, "RDPGFX ClearCodec", "production_encoder", true)
	assertEntryHasString(t, coverage.RuntimeEmitters, "RDPGFX AVC444", "client_proof", "missing-production-client-proof")
	assertEntryHasBool(t, coverage.RuntimeEmitters, "RDPGFX AVC444", "production_encoder", true)
	assertEntryHasBool(t, coverage.RuntimeEmitters, "RDPGFX Progressive / other progressive codecs", "production_encoder", true)
	assertEntryHasString(t, coverage.UpstreamMetadata, "RDPGFX AVC444 / AVC444v2", "android_emitter", "partial-production-opt-in")
	assertEntryHasString(t, coverage.UpstreamMetadata, "RDPGFX Progressive / other progressive codecs", "android_emitter", "partial-production-opt-in")
	assertStringPresent(t, coverage.ReleaseDefaults, "RDPGFX Planar")
	assertStringPresent(t, coverage.NonDefaultExperimentalEmitters, "PNG bitmap codec")
	assertStringPresent(t, coverage.NonDefaultExperimentalEmitters, "RDPGFX ClearCodec")
}

func extractHereDoc(t *testing.T, text, start, end string) string {
	t.Helper()
	startIdx := strings.Index(text, start)
	if startIdx < 0 {
		t.Fatalf("start marker %q not found", start)
	}
	afterStart := text[startIdx+len(start):]
	if strings.HasPrefix(afterStart, "\r\n") {
		afterStart = afterStart[2:]
	} else if strings.HasPrefix(afterStart, "\n") {
		afterStart = afterStart[1:]
	}
	endMarker := "\n" + end
	endIdx := strings.Index(afterStart, endMarker)
	if endIdx < 0 {
		t.Fatalf("end marker %q not found", end)
	}
	return afterStart[:endIdx]
}

func assertEntryNamed(t *testing.T, entries []map[string]any, name string) {
	t.Helper()
	for _, entry := range entries {
		if entry["name"] == name {
			return
		}
	}
	t.Fatalf("entry %q not found in %#v", name, entries)
}

func assertEntryHasBool(t *testing.T, entries []map[string]any, name, key string, want bool) {
	t.Helper()
	for _, entry := range entries {
		if entry["name"] == name {
			got, ok := entry[key].(bool)
			if !ok || got != want {
				t.Fatalf("entry %q key %q = %#v, want %t", name, key, entry[key], want)
			}
			return
		}
	}
	t.Fatalf("entry %q not found in %#v", name, entries)
}

func assertEntryHasString(t *testing.T, entries []map[string]any, name, key, want string) {
	t.Helper()
	for _, entry := range entries {
		if entry["name"] == name {
			got, ok := entry[key].(string)
			if !ok || got != want {
				t.Fatalf("entry %q key %q = %#v, want %q", name, key, entry[key], want)
			}
			return
		}
	}
	t.Fatalf("entry %q not found in %#v", name, entries)
}

func assertStringPresent(t *testing.T, entries []string, want string) {
	t.Helper()
	for _, entry := range entries {
		if entry == want {
			return
		}
	}
	t.Fatalf("string %q not found in %#v", want, entries)
}
