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
	ExitCode       string   `json:"exit_code"`
	TCPSeen        bool     `json:"tcp_seen"`
	X224Seen       bool     `json:"x224_seen"`
	MCSSeen        bool     `json:"mcs_seen"`
	BitmapSeen     bool     `json:"bitmap_seen"`
	RDPGFXSeen     bool     `json:"rdpgfx_seen"`
	H264StatusSeen bool     `json:"h264_status_seen"`
	H264WriteSeen  bool     `json:"h264_write_seen"`
	H264WriteCount int      `json:"h264_write_count,omitempty"`
	H264WriteBytes int      `json:"h264_write_bytes,omitempty"`
	H264Reason     string   `json:"h264_reason,omitempty"`
	AVC420ExitCode string   `json:"avc420_exit_code,omitempty"`
	ActiveSeen     bool     `json:"active_seen"`
	FastPathSeen   bool     `json:"fastpath_seen"`
	ErrorLines     []string `json:"error_lines"`
	ServerPhases   []string `json:"server_phases"`
	ScreenshotPNG  bool     `json:"screenshot_png"`
	ScreenshotXWD  bool     `json:"screenshot_xwd"`
	FreeRDPLogSize int      `json:"freerdp_log_size"`
	ServerLogSize  int      `json:"server_log_size"`
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
	s.RDPGFXSeen = strings.Contains(sv, "rdpgfx_caps_confirm") || strings.Contains(sv, "rdpgfx_caps_advertise") || strings.Contains(sv, "Microsoft::Windows::RDS::Graphics")
	s.H264StatusSeen = strings.Contains(sv, "rdpgfx_h264_status")
	s.H264WriteSeen = strings.Contains(sv, "rdpgfx_h264_write")
	s.H264WriteCount, s.H264WriteBytes = traceCountAndSum(sv, "rdpgfx_h264_write", "bytes")
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
		"- RDPGFX trace seen: `%v`\n"+
		"- H.264 status trace seen: `%v`\n"+
		"- H.264 write trace seen: `%v`\n"+
		"- H.264 write trace count: `%d`\n"+
		"- H.264 write trace bytes: `%d`\n"+
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
		s.ExitCode, s.TCPSeen, s.X224Seen, s.MCSSeen, s.BitmapSeen, s.RDPGFXSeen, s.H264StatusSeen, s.H264WriteSeen, s.H264WriteCount, s.H264WriteBytes, s.H264Reason, s.AVC420ExitCode, s.ActiveSeen, s.FastPathSeen, s.FreeRDPLogSize, s.ServerLogSize, s.ScreenshotPNG, s.ScreenshotXWD, bullet(s.ServerPhases), bullet(s.ErrorLines))
	must(os.WriteFile(filepath.Join(dir, "summary.md"), []byte(md), 0o644))
	fmt.Println("wrote FreeRDP summaries")
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
