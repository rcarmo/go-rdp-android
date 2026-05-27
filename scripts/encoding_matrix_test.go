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
		"run_case rdpgfx-progressive-fixture",
		"-progressive-file $OUT/codec-fixture.bin",
		"run_case rdpgfx-avc444-fixture",
		"-avc444-file $OUT/codec-fixture.bin",
		"run_case rdpgfx-avc444v2-fixture",
		"-avc444v2-file $OUT/codec-fixture.bin",
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
	if len(coverage.MissingRuntimeEmitters) == 0 {
		t.Fatal("missing_runtime_emitters is empty")
	}
	if len(coverage.ReleaseDefaults) == 0 {
		t.Fatal("release_defaults is empty")
	}
	if len(coverage.NonDefaultExperimentalEmitters) == 0 {
		t.Fatal("non_default_experimental_emitters is empty")
	}
	assertEntryNamed(t, coverage.RuntimeEmitters, "PNG bitmap codec")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX Planar")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RemoteFX / RFX")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX ClearCodec")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX Progressive / other progressive codecs")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX AVC444")
	assertEntryNamed(t, coverage.RuntimeEmitters, "RDPGFX AVC444v2")
	assertEntryNamed(t, coverage.MissingRuntimeEmitters, "RDPGFX Progressive / other progressive codecs")
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

func assertStringPresent(t *testing.T, entries []string, want string) {
	t.Helper()
	for _, entry := range entries {
		if entry == want {
			return
		}
	}
	t.Fatalf("string %q not found in %#v", want, entries)
}
