//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type summary struct {
	ExitCode                  string   `json:"exit_code"`
	TCPSeen                   bool     `json:"tcp_seen"`
	X224Seen                  bool     `json:"x224_seen"`
	MCSSeen                   bool     `json:"mcs_seen"`
	BitmapSeen                bool     `json:"bitmap_seen"`
	BitmapRLESeen             bool     `json:"bitmap_rle_seen"`
	BitmapRLECount            int      `json:"bitmap_rle_count,omitempty"`
	BitmapRLEBytes            int      `json:"bitmap_rle_bytes,omitempty"`
	BitmapRLESavedBytes       int      `json:"bitmap_rle_saved_bytes,omitempty"`
	NSCodecSelected           bool     `json:"nscodec_selected,omitempty"`
	NSCodecWriteSeen          bool     `json:"nscodec_write_seen,omitempty"`
	NSCodecWriteCount         int      `json:"nscodec_write_count,omitempty"`
	NSCodecWriteBytes         int      `json:"nscodec_write_bytes,omitempty"`
	JPEGCodecSelected         bool     `json:"jpeg_codec_selected,omitempty"`
	JPEGCodecWriteSeen        bool     `json:"jpeg_codec_write_seen,omitempty"`
	JPEGCodecWriteCount       int      `json:"jpeg_codec_write_count,omitempty"`
	JPEGCodecWriteBytes       int      `json:"jpeg_codec_write_bytes,omitempty"`
	RFXCodecSelected          bool     `json:"rfx_codec_selected,omitempty"`
	RDPGFXClearCodecSelected  bool     `json:"rdpgfx_clearcodec_selected,omitempty"`
	RDPGFXProgressiveSelected bool     `json:"rdpgfx_progressive_selected,omitempty"`
	RDPGFXAVC444Selected      bool     `json:"rdpgfx_avc444_selected,omitempty"`
	RDPGFXAVC444v2Selected    bool     `json:"rdpgfx_avc444v2_selected,omitempty"`
	RDPGFXSeen                bool     `json:"rdpgfx_seen"`
	H264StatusSeen            bool     `json:"h264_status_seen"`
	H264WriteSeen             bool     `json:"h264_write_seen"`
	H264WriteCount            int      `json:"h264_write_count,omitempty"`
	H264WriteBytes            int      `json:"h264_write_bytes,omitempty"`
	H264Ready                 string   `json:"h264_ready,omitempty"`
	H264Version               string   `json:"h264_version,omitempty"`
	H264Flags                 string   `json:"h264_flags,omitempty"`
	H264Reason                string   `json:"h264_reason,omitempty"`
	AVC420ExitCode            string   `json:"avc420_exit_code,omitempty"`
	ActiveSeen                bool     `json:"active_seen"`
	FastPathSeen              bool     `json:"fastpath_seen"`
	ErrorLines                []string `json:"error_lines"`
	ServerPhases              []string `json:"server_phases"`
	ScreenshotPNG             bool     `json:"screenshot_png"`
	ScreenshotXWD             bool     `json:"screenshot_xwd"`
	FreeRDPLogSize            int      `json:"freerdp_log_size"`
	ServerLogSize             int      `json:"server_log_size"`
}

func main() {
	dir := "freerdp-artifacts"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	s := summary{ExitCode: readTrim(filepath.Join(dir, "exit-code.txt"))}
	xf := read(filepath.Join(dir, "xfreerdp.log"))
	sv := read(filepath.Join(dir, "mock-server.log"))
	s.FreeRDPLogSize = len(xf)
	s.ServerLogSize = len(sv)
	s.TCPSeen = strings.Contains(sv, "rdp initial handshake") || strings.Contains(xf, "127.0.0.1")
	s.X224Seen = strings.Contains(sv, "x224") || strings.Contains(sv, "initial handshake")
	s.MCSSeen = strings.Contains(sv, "MCS") || strings.Contains(sv, "mcs_")
	s.BitmapSeen = strings.Contains(xf, "Bitmap Update Data PDU") || strings.Contains(xf, "recv Update Data PDU")
	s.BitmapRLESeen = strings.Contains(sv, "bitmap_rle_")
	s.BitmapRLECount, s.BitmapRLEBytes, s.BitmapRLESavedBytes = bitmapRLETraceStats(sv)
	s.NSCodecSelected = strings.Contains(sv, "nscodec_selected")
	s.NSCodecWriteSeen = strings.Contains(sv, "nscodec_write")
	s.NSCodecWriteCount, s.NSCodecWriteBytes = traceCountAndSum(sv, "nscodec_write", "bytes")
	s.JPEGCodecSelected = strings.Contains(sv, "jpeg_codec_selected")
	s.JPEGCodecWriteSeen = strings.Contains(sv, "jpeg_codec_write")
	s.JPEGCodecWriteCount, s.JPEGCodecWriteBytes = traceCountAndSum(sv, "jpeg_codec_write", "bytes")
	s.RFXCodecSelected = strings.Contains(sv, "rfx_codec_selected")
	s.RDPGFXClearCodecSelected = strings.Contains(sv, "rdpgfx_clearcodec_selected")
	s.RDPGFXProgressiveSelected = strings.Contains(sv, "rdpgfx_progressive_selected")
	s.RDPGFXAVC444Selected = strings.Contains(sv, "rdpgfx_avc444_selected")
	s.RDPGFXAVC444v2Selected = strings.Contains(sv, "rdpgfx_avc444v2_selected")
	s.RDPGFXSeen = strings.Contains(sv, "rdpgfx_caps_confirm") || strings.Contains(sv, "rdpgfx_caps_advertise") || strings.Contains(sv, "Microsoft::Windows::RDS::Graphics")
	s.H264StatusSeen = strings.Contains(sv, "rdpgfx_h264_status")
	s.H264WriteSeen = strings.Contains(sv, "rdpgfx_h264_write")
	s.H264WriteCount, s.H264WriteBytes = traceCountAndSum(sv, "rdpgfx_h264_write", "bytes")
	s.H264Ready = lastTraceValue(sv, "rdpgfx_h264_status", "ready")
	s.H264Version = lastTraceValue(sv, "rdpgfx_h264_status", "version")
	s.H264Flags = lastTraceValue(sv, "rdpgfx_h264_status", "flags")
	s.H264Reason = lastTraceValue(sv, "rdpgfx_h264_status", "reason")
	s.AVC420ExitCode = readTrim(filepath.Join(dir, "avc420-exit-code.txt"))
	if s.AVC420ExitCode == "unknown" {
		s.AVC420ExitCode = ""
	}
	s.ActiveSeen = strings.Contains(xf, "CONNECTION_STATE_ACTIVE")
	s.FastPathSeen = strings.Contains(sv, "fastpath_ignore") || strings.Contains(sv, "fastpath_input")
	s.ScreenshotPNG = exists(filepath.Join(dir, "xfreerdp-root.png"))
	s.ScreenshotXWD = exists(filepath.Join(dir, "xfreerdp-root.xwd"))
	for _, line := range strings.Split(xf, "\n") {
		lo := strings.ToLower(line)
		if strings.Contains(lo, "error") || strings.Contains(lo, "fail") || strings.Contains(lo, "warn") {
			s.ErrorLines = appendLimited(s.ErrorLines, line, 40)
		}
	}
	for _, line := range strings.Split(sv, "\n") {
		if strings.Contains(line, "trace phase=") {
			s.ServerPhases = appendLimited(s.ServerPhases, line, 60)
		}
	}
	jsonData, err := json.MarshalIndent(s, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(dir, "summary.json"), append(jsonData, '\n'), 0o644))
	md := fmt.Sprintf("# FreeRDP compatibility probe\n\n"+
		"- Exit code: `%s`\n"+
		"- TCP seen: `%v`\n"+
		"- X.224 seen: `%v`\n"+
		"- MCS seen: `%v`\n"+
		"- Bitmap/update trace seen: `%v`\n"+
		"- Bitmap RLE trace seen: `%v`\n"+
		"- Bitmap RLE trace count: `%d`\n"+
		"- Bitmap RLE bytes: `%d`\n"+
		"- Bitmap RLE saved bytes: `%d`\n"+
		"- NSCodec selected trace seen: `%v`\n"+
		"- NSCodec write trace seen: `%v`\n"+
		"- NSCodec write trace count: `%d`\n"+
		"- NSCodec write trace bytes: `%d`\n"+
		"- JPEG codec selected trace seen: `%v`\n"+
		"- JPEG codec write trace seen: `%v`\n"+
		"- JPEG codec write trace count: `%d`\n"+
		"- JPEG codec write trace bytes: `%d`\n"+
		"- RDPGFX trace seen: `%v`\n"+
		"- H.264 status trace seen: `%v`\n"+
		"- H.264 write trace seen: `%v`\n"+
		"- H.264 write trace count: `%d`\n"+
		"- H.264 write trace bytes: `%d`\n"+
		"- H.264 ready: `%s`\n"+
		"- H.264 version: `%s`\n"+
		"- H.264 flags: `%s`\n"+
		"- H.264 status reason: `%s`\n"+
		"- FreeRDP `/gfx:AVC420` exit code: `%s`\n"+
		"- FreeRDP active state seen: `%v`\n"+
		"- Fast-path packet handling seen: `%v`\n"+
		"- FreeRDP log bytes: `%d`\n"+
		"- Server log bytes: `%d`\n"+
		"- PNG screenshot: `%v`\n"+
		"- XWD screenshot: `%v`\n\n"+
		"## Recent server trace phases\n\n%s\n\n"+
		"## FreeRDP warning/error lines\n\n%s\n",
		s.ExitCode, s.TCPSeen, s.X224Seen, s.MCSSeen, s.BitmapSeen, s.BitmapRLESeen, s.BitmapRLECount, s.BitmapRLEBytes, s.BitmapRLESavedBytes, s.NSCodecSelected, s.NSCodecWriteSeen, s.NSCodecWriteCount, s.NSCodecWriteBytes, s.JPEGCodecSelected, s.JPEGCodecWriteSeen, s.JPEGCodecWriteCount, s.JPEGCodecWriteBytes, s.RDPGFXSeen, s.H264StatusSeen, s.H264WriteSeen, s.H264WriteCount, s.H264WriteBytes, s.H264Ready, s.H264Version, s.H264Flags, s.H264Reason, s.AVC420ExitCode, s.ActiveSeen, s.FastPathSeen, s.FreeRDPLogSize, s.ServerLogSize, s.ScreenshotPNG, s.ScreenshotXWD, bullet(s.ServerPhases), bullet(s.ErrorLines))
	must(os.WriteFile(filepath.Join(dir, "summary.md"), []byte(md), 0o644))
	fmt.Println("wrote FreeRDP summaries")
}

func bitmapRLETraceStats(logText string) (int, int, int) {
	count := 0
	bytes := 0
	uncompressedBytes := 0
	for _, phase := range []string{"bitmap_rle_tile", "bitmap_rle_solid"} {
		phaseCount, phaseBytes := traceCountAndSum(logText, phase, "bytes")
		_, phaseUncompressed := traceCountAndSum(logText, phase, "uncompressed_bytes")
		count += phaseCount
		bytes += phaseBytes
		uncompressedBytes += phaseUncompressed
	}
	saved := uncompressedBytes - bytes
	if saved < 0 {
		saved = 0
	}
	return count, bytes, saved
}

func traceCountAndSum(logText, phase, key string) (int, int) {
	needle := "trace phase=" + phase
	prefix := key + "="
	count := 0
	sum := 0
	for _, line := range strings.Split(logText, "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		count++
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, prefix) {
				if n, err := strconv.Atoi(strings.TrimPrefix(field, prefix)); err == nil {
					sum += n
				}
			}
		}
	}
	return count, sum
}

func lastTraceValue(logText, phase, key string) string {
	needle := "trace phase=" + phase
	prefix := key + "="
	value := ""
	for _, line := range strings.Split(logText, "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, prefix) {
				value = strings.TrimPrefix(field, prefix)
			}
		}
	}
	return value
}

func read(path string) string { b, _ := os.ReadFile(path); return string(b) }
func readTrim(path string) string {
	v := strings.TrimSpace(read(path))
	if v == "" {
		return "unknown"
	}
	return v
}
func exists(path string) bool { st, err := os.Stat(path); return err == nil && st.Size() > 0 }
func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
func appendLimited(v []string, s string, max int) []string {
	if strings.TrimSpace(s) == "" {
		return v
	}
	v = append(v, s)
	if len(v) > max {
		return v[len(v)-max:]
	}
	return v
}
func bullet(lines []string) string {
	if len(lines) == 0 {
		return "- none\n"
	}
	return "- " + strings.Join(lines, "\n- ") + "\n"
}
