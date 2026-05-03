//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type summary struct {
	ExitCode       string   `json:"exit_code"`
	TCPSeen        bool     `json:"tcp_seen"`
	X224Seen       bool     `json:"x224_seen"`
	MCSSeen        bool     `json:"mcs_seen"`
	BitmapSeen     bool     `json:"bitmap_seen"`
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
	s.BitmapSeen = strings.Contains(sv, "bitmap") || strings.Contains(sv, "Bitmap")
	s.ActiveSeen = strings.Contains(xf, "CONNECTION_STATE_ACTIVE")
	s.FastPathSeen = strings.Contains(sv, "fastpath_ignore")
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
	jsonData, _ := json.MarshalIndent(s, "", "  ")
	must(os.WriteFile(filepath.Join(dir, "summary.json"), append(jsonData, '\n'), 0o644))
	md := fmt.Sprintf("# FreeRDP compatibility probe\n\n"+
		"- Exit code: `%s`\n"+
		"- TCP seen: `%v`\n"+
		"- X.224 seen: `%v`\n"+
		"- MCS seen: `%v`\n"+
		"- Bitmap/update trace seen: `%v`\n"+
		"- FreeRDP active state seen: `%v`\n"+
		"- Fast-path packet handling seen: `%v`\n"+
		"- FreeRDP log bytes: `%d`\n"+
		"- Server log bytes: `%d`\n"+
		"- PNG screenshot: `%v`\n"+
		"- XWD screenshot: `%v`\n\n"+
		"## Recent server trace phases\n\n%s\n\n"+
		"## FreeRDP warning/error lines\n\n%s\n",
		s.ExitCode, s.TCPSeen, s.X224Seen, s.MCSSeen, s.BitmapSeen, s.ActiveSeen, s.FastPathSeen, s.FreeRDPLogSize, s.ServerLogSize, s.ScreenshotPNG, s.ScreenshotXWD, bullet(s.ServerPhases), bullet(s.ErrorLines))
	must(os.WriteFile(filepath.Join(dir, "summary.md"), []byte(md), 0o644))
	fmt.Println("wrote FreeRDP summaries")
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
		panic(err)
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
