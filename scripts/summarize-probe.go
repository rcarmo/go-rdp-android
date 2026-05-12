//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type probeSummary struct {
	BitmapUpdates        int     `json:"bitmap_updates"`
	PacketsRead          int     `json:"packets_read"`
	PacketsWritten       int     `json:"packets_written"`
	BytesRead            int64   `json:"bytes_read"`
	BytesWritten         int64   `json:"bytes_written"`
	BitmapPayloadBytes   int64   `json:"bitmap_payload_bytes"`
	BitmapRectangles     int     `json:"bitmap_rectangles"`
	BitmapPixels         int64   `json:"bitmap_pixels"`
	DurationMs           int64   `json:"duration_ms"`
	HandshakeMs          int64   `json:"handshake_ms"`
	BitmapReadMs         int64   `json:"bitmap_read_ms"`
	FirstBitmapMs        int64   `json:"first_bitmap_ms"`
	ReadThroughputMbps   float64 `json:"read_throughput_mbps"`
	BitmapThroughputMbps float64 `json:"bitmap_throughput_mbps"`
	AverageUpdateBytes   float64 `json:"average_update_bytes"`
	AverageUpdateMs      float64 `json:"average_update_ms"`
	ScreenshotPath       string  `json:"screenshot_path,omitempty"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: go run ./scripts/summarize-probe.go <summary.json> <summary.md>\n")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fatalf("read summary: %v", err)
	}
	var s probeSummary
	if err := json.Unmarshal(data, &s); err != nil {
		fatalf("parse summary: %v", err)
	}
	md := fmt.Sprintf(`# Probe summary

| Metric | Value |
| --- | ---: |
| Bitmap updates | %d |
| Bitmap rectangles | %d |
| Bitmap pixels | %d |
| Bitmap payload bytes | %d |
| Packets read | %d |
| Packets written | %d |
| Bytes read | %d |
| Bytes written | %d |
| Duration | %d ms |
| Handshake | %d ms |
| Bitmap read | %d ms |
| First bitmap | %d ms |
| Read throughput | %.3f Mbps |
| Bitmap throughput | %.3f Mbps |
| Average update bytes | %.1f |
| Average update time | %.1f ms |
`, s.BitmapUpdates, s.BitmapRectangles, s.BitmapPixels, s.BitmapPayloadBytes, s.PacketsRead, s.PacketsWritten, s.BytesRead, s.BytesWritten, s.DurationMs, s.HandshakeMs, s.BitmapReadMs, s.FirstBitmapMs, s.ReadThroughputMbps, s.BitmapThroughputMbps, s.AverageUpdateBytes, s.AverageUpdateMs)
	if s.ScreenshotPath != "" {
		md += fmt.Sprintf("\n- Screenshot: `%s`\n", s.ScreenshotPath)
	}
	if err := os.WriteFile(os.Args[2], []byte(md), 0o600); err != nil {
		fatalf("write summary: %v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
