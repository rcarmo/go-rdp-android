package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
	"unicode/utf16"

	rdpauth "github.com/rcarmo/go-rdp/pkg/auth"
)

var traceOut atomic.Value
var dumpPackets atomic.Bool

type probeSummary struct {
	BitmapUpdates        int                 `json:"bitmap_updates"`
	PacketsRead          int                 `json:"packets_read"`
	PacketsWritten       int                 `json:"packets_written"`
	BytesRead            int64               `json:"bytes_read"`
	BytesWritten         int64               `json:"bytes_written"`
	BitmapPayloadBytes   int64               `json:"bitmap_payload_bytes"`
	BitmapRectangles     int                 `json:"bitmap_rectangles"`
	BitmapPixels         int64               `json:"bitmap_pixels"`
	DurationMs           int64               `json:"duration_ms"`
	HandshakeMs          int64               `json:"handshake_ms"`
	BitmapReadMs         int64               `json:"bitmap_read_ms"`
	FirstBitmapMs        int64               `json:"first_bitmap_ms"`
	ReadThroughputMbps   float64             `json:"read_throughput_mbps"`
	BitmapThroughputMbps float64             `json:"bitmap_throughput_mbps"`
	AverageUpdateBytes   float64             `json:"average_update_bytes"`
	AverageUpdateMs      float64             `json:"average_update_ms"`
	ScreenshotWidth      int                 `json:"screenshot_width,omitempty"`
	ScreenshotHeight     int                 `json:"screenshot_height,omitempty"`
	ScreenshotPath       string              `json:"screenshot_path,omitempty"`
	Scenes               []probeSceneSummary `json:"scenes,omitempty"`
}

type probeScenePlan struct {
	Name       string             `json:"name"`
	Command    string             `json:"command,omitempty"`
	Actions    []probeSceneAction `json:"actions,omitempty"`
	WaitMs     int                `json:"wait_ms,omitempty"`
	MaxUpdates int                `json:"max_updates,omitempty"`
}

type probeSceneAction struct {
	Type     string `json:"type"`
	DelayMs  int    `json:"delay_ms,omitempty"`
	Scancode uint16 `json:"scancode,omitempty"`
	X        uint16 `json:"x,omitempty"`
	Y        uint16 `json:"y,omitempty"`
}

type probeSceneSummary struct {
	Name               string  `json:"name"`
	Command            string  `json:"command,omitempty"`
	Updates            int     `json:"updates"`
	BytesRead          int64   `json:"bytes_read"`
	BitmapPayloadBytes int64   `json:"bitmap_payload_bytes"`
	BitmapRectangles   int     `json:"bitmap_rectangles"`
	BitmapPixels       int64   `json:"bitmap_pixels"`
	DurationMs         int64   `json:"duration_ms"`
	FirstUpdateMs      int64   `json:"first_update_ms,omitempty"`
	IdleTimeoutMs      int     `json:"idle_timeout_ms"`
	ThroughputMbps     float64 `json:"throughput_mbps"`
	ScreenshotPath     string  `json:"screenshot_path"`
}

var summary probeSummary

func main() {
	started := time.Now()
	addr := flag.String("addr", "127.0.0.1:3390", "RDP server address")
	traceDir := flag.String("trace-dir", "", "directory for client/server packet hex traces")
	summaryPath := flag.String("summary", "", "write JSON probe summary")
	updates := flag.Int("updates", 1, "number of bitmap update packets to read after FontMap")
	screenshotPath := flag.String("screenshot", "", "compose bitmap updates into a PNG screenshot")
	screenshotWidth := flag.Int("screenshot-width", 320, "screenshot canvas width")
	screenshotHeight := flag.Int("screenshot-height", 240, "screenshot canvas height")
	username := flag.String("username", "", "username to send in Client Info")
	password := flag.String("password", "", "password to send in Client Info")
	domain := flag.String("domain", "", "domain to send in Client Info")
	dump := flag.Bool("dump-packets", true, "print full packet hex dumps")
	warmupUpdates := flag.Int("warmup-updates", 0, "number of initial bitmap updates to consume before scene commands")
	warmupScreenshot := flag.String("warmup-screenshot", "", "write screenshot after warmup updates")
	scenePlanPath := flag.String("scene-plan", "", "JSON scene plan for single-session navigation/capture")
	artifactDir := flag.String("artifact-dir", ".", "directory for scene screenshots")
	sceneIdleTimeout := flag.Int("scene-idle-timeout-ms", 1500, "scene capture stops after this read-idle timeout")
	sceneMaxUpdates := flag.Int("scene-max-updates", 420, "maximum bitmap updates to read per scene")
	allowPartial := flag.Bool("allow-partial", false, "allow EOF/timeout before requested bitmap updates and still write artifacts")
	nla := flag.Bool("nla", false, "request Hybrid/NLA and perform CredSSP before MCS")
	flag.Parse()
	dumpPackets.Store(*dump)
	if *traceDir != "" {
		if err := os.MkdirAll(*traceDir, 0o755); err != nil {
			log.Fatal(err)
		}
		traceOut.Store(*traceDir)
	}

	conn, err := net.DialTimeout("tcp", *addr, 3*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := sendX224ConnectionRequest(conn, *nla); err != nil {
		log.Fatal(err)
	}
	resp, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("X.224 confirm: %x\n", resp)
	selectedProtocol := parseSelectedProtocol(resp)
	if selectedProtocol == 0x00000001 || selectedProtocol == 0x00000002 {
		tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
		if err := tlsConn.Handshake(); err != nil {
			log.Fatal(err)
		}
		conn = tlsConn
		defer conn.Close()
		if selectedProtocol == 0x00000002 {
			if err := performClientCredSSP(conn, tlsConn, *username, *password, *domain); err != nil {
				log.Fatal(err)
			}
			fmt.Println("completed CredSSP/NLA")
		}
	}

	if err := sendMCSConnectInitial(conn); err != nil {
		log.Fatal(err)
	}
	mcsResp, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("MCS response: %x\n", mcsResp)

	if err := sendMCSDomainPDU(conn, 1, []byte{1, 0, 1, 0}); err != nil {
		log.Fatal(err)
	}
	fmt.Println("sent ErectDomainRequest")

	if err := sendMCSDomainPDU(conn, 10, nil); err != nil {
		log.Fatal(err)
	}
	attachResp, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("AttachUserConfirm: %x\n", attachResp)

	joinBody := append(encodePERInteger16(1001, 1001), encodePERInteger16(1003, 0)...)
	if err := sendMCSDomainPDU(conn, 14, joinBody); err != nil {
		log.Fatal(err)
	}
	joinResp, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ChannelJoinConfirm: %x\n", joinResp)

	clientInfo := append([]byte{0x40, 0, 0, 0}, buildClientInfoPayload(*username, *password, *domain)...)
	if err := sendMCSDomainPDU(conn, 25, buildMCSSendDataRequest(1001, 1003, clientInfo)); err != nil {
		log.Fatal(err)
	}
	fmt.Println("sent minimal Client Info security PDU")
	demand, err := readDemandActiveOrSkipLicense(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("DemandActive: %x\n", demand)

	confirm := buildConfirmActivePDU(0x000103ea, 1001)
	if err := sendMCSDomainPDU(conn, 25, buildMCSSendDataRequest(1001, 1003, confirm)); err != nil {
		log.Fatal(err)
	}
	fmt.Println("sent minimal Confirm Active PDU")

	if err := sendShareData(conn, 0x1f, synchronizePayload()); err != nil {
		log.Fatal(err)
	}
	readAndPrint(conn, "Server Synchronize")
	if err := sendShareData(conn, 0x14, controlPayload(0x0004)); err != nil {
		log.Fatal(err)
	}
	readAndPrint(conn, "Server Control Cooperate")
	if err := sendShareData(conn, 0x14, controlPayload(0x0001)); err != nil {
		log.Fatal(err)
	}
	readAndPrint(conn, "Server Control Granted")
	if err := sendShareData(conn, 0x27, []byte{0, 0, 0, 0, 3, 0, 0x32, 0}); err != nil {
		log.Fatal(err)
	}
	readAndPrint(conn, "Server FontMap")
	handshakeDone := time.Now()
	summary.HandshakeMs = handshakeDone.Sub(started).Milliseconds()
	var screenshot *image.RGBA
	if *screenshotPath != "" {
		summary.ScreenshotWidth = *screenshotWidth
		summary.ScreenshotHeight = *screenshotHeight
		summary.ScreenshotPath = *screenshotPath
		screenshot = image.NewRGBA(image.Rect(0, 0, *screenshotWidth, *screenshotHeight))
	}
	bitmapStarted := time.Now()
	if *scenePlanPath != "" {
		if screenshot == nil {
			screenshot = image.NewRGBA(image.Rect(0, 0, *screenshotWidth, *screenshotHeight))
		}
		if *warmupUpdates > 0 {
			if err := readBitmapUpdates(conn, *warmupUpdates, screenshot); err != nil {
				log.Fatal(err)
			}
		}
		if *warmupScreenshot != "" {
			if err := writePNG(*warmupScreenshot, screenshot); err != nil {
				log.Fatal(err)
			}
		}
		if err := runScenePlan(conn, *scenePlanPath, *artifactDir, screenshot, *sceneIdleTimeout, *sceneMaxUpdates); err != nil {
			log.Fatal(err)
		}
	} else {
		for i := 0; i < *updates; i++ {
			pkt, err := readTPKT(conn)
			if err != nil {
				if *allowPartial && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) {
					break
				}
				log.Fatal(err)
			}
			if dumpPackets.Load() {
				fmt.Printf("Server Bitmap Update %d: %x\n", i+1, pkt)
			} else {
				fmt.Printf("Server Bitmap Update %d: %d bytes\n", i+1, len(pkt))
			}
			if i == 0 {
				summary.FirstBitmapMs = time.Since(started).Milliseconds()
			}
			if screenshot != nil {
				stats, err := applyBitmapUpdatePacket(screenshot, pkt)
				if err != nil {
					fmt.Fprintf(os.Stderr, "bitmap decode warning: %v\n", err)
				} else {
					summary.BitmapPayloadBytes += int64(stats.Bytes)
					summary.BitmapRectangles += stats.Rectangles
					summary.BitmapPixels += int64(stats.Pixels)
				}
			}
			summary.BitmapUpdates++
		}
		if screenshot != nil {
			if err := writePNG(*screenshotPath, screenshot); err != nil {
				log.Fatal(err)
			}
		}
	}
	summary.DurationMs = time.Since(started).Milliseconds()
	summary.BitmapReadMs = time.Since(bitmapStarted).Milliseconds()
	if summary.DurationMs > 0 {
		summary.ReadThroughputMbps = mbps(summary.BytesRead, summary.DurationMs)
	}
	if summary.BitmapReadMs > 0 {
		summary.BitmapThroughputMbps = mbps(summary.BitmapPayloadBytes, summary.BitmapReadMs)
	}
	if summary.BitmapUpdates > 0 {
		summary.AverageUpdateBytes = float64(summary.BytesRead) / float64(summary.BitmapUpdates)
		summary.AverageUpdateMs = float64(summary.BitmapReadMs) / float64(summary.BitmapUpdates)
	}
	if *summaryPath != "" {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*summaryPath, append(data, '\n'), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

func readBitmapUpdates(conn net.Conn, count int, screenshot *image.RGBA) error {
	for i := 0; i < count; i++ {
		pkt := readAndPrint(conn, fmt.Sprintf("Warmup Bitmap Update %d", i+1))
		stats, err := applyBitmapUpdatePacket(screenshot, pkt)
		if err != nil {
			return err
		}
		summary.BitmapUpdates++
		summary.BitmapPayloadBytes += int64(stats.Bytes)
		summary.BitmapRectangles += stats.Rectangles
		summary.BitmapPixels += int64(stats.Pixels)
	}
	return nil
}

func runScenePlan(conn net.Conn, path, artifactDir string, screenshot *image.RGBA, idleTimeoutMs, defaultMaxUpdates int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var scenes []probeScenePlan
	if err := json.Unmarshal(data, &scenes); err != nil {
		return err
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}
	for _, scene := range scenes {
		if scene.Name == "" {
			return fmt.Errorf("scene without name")
		}
		var cmd *exec.Cmd
		if scene.Command != "" {
			cmd = exec.Command("sh", "-c", scene.Command)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("scene %s command failed to start: %w", scene.Name, err)
			}
		}
		actionErr := make(chan error, 1)
		go func() { actionErr <- runRDPSceneActions(conn, scene.Actions) }()
		if scene.WaitMs > 0 {
			time.Sleep(time.Duration(scene.WaitMs) * time.Millisecond)
		}
		maxUpdates := scene.MaxUpdates
		if maxUpdates <= 0 {
			maxUpdates = defaultMaxUpdates
		}
		shot := filepath.Join(artifactDir, "rdp-"+scene.Name+".png")
		sceneSummary, err := captureScene(conn, scene, screenshot, shot, idleTimeoutMs, maxUpdates)
		if cmd != nil {
			if waitErr := cmd.Wait(); waitErr != nil && err == nil {
				err = fmt.Errorf("scene %s command failed: %w", scene.Name, waitErr)
			}
		}
		if actionWaitErr := <-actionErr; actionWaitErr != nil && err == nil {
			err = fmt.Errorf("scene %s RDP action failed: %w", scene.Name, actionWaitErr)
		}
		if err != nil {
			return err
		}
		summary.Scenes = append(summary.Scenes, sceneSummary)
	}
	return nil
}

func runRDPSceneActions(conn net.Conn, actions []probeSceneAction) error {
	for _, action := range actions {
		if action.DelayMs > 0 {
			time.Sleep(time.Duration(action.DelayMs) * time.Millisecond)
		}
		switch action.Type {
		case "key-home":
			if err := sendRDPKey(conn, 0x47); err != nil {
				return err
			}
		case "key":
			if err := sendRDPKey(conn, action.Scancode); err != nil {
				return err
			}
		case "tap":
			if err := sendRDPTap(conn, action.X, action.Y); err != nil {
				return err
			}
		case "wait", "":
			// delay-only action.
		default:
			return fmt.Errorf("unknown RDP action type %q", action.Type)
		}
	}
	return nil
}

func sendRDPKey(conn net.Conn, scancode uint16) error {
	if err := sendShareData(conn, 0x1c, buildSlowPathInputPDU(
		buildSlowPathInputEvent(0x0004, 0, scancode, 0),
	)); err != nil {
		return err
	}
	return sendShareData(conn, 0x1c, buildSlowPathInputPDU(
		buildSlowPathInputEvent(0x0004, 0x8000, scancode, 0),
	))
}

func sendRDPTap(conn net.Conn, x, y uint16) error {
	return sendShareData(conn, 0x1c, buildSlowPathInputPDU(
		buildSlowPathInputEvent(0x8001, 0x0800, x, y),
		buildSlowPathInputEvent(0x8001, 0x9000, x, y),
		buildSlowPathInputEvent(0x8001, 0x1000, x, y),
	))
}

func captureScene(conn net.Conn, scene probeScenePlan, screenshot *image.RGBA, shot string, idleTimeoutMs, maxUpdates int) (probeSceneSummary, error) {
	started := time.Now()
	beforeBytes := summary.BytesRead
	firstUpdateMs := int64(0)
	out := probeSceneSummary{Name: scene.Name, Command: scene.Command, IdleTimeoutMs: idleTimeoutMs, ScreenshotPath: shot}
	for out.Updates < maxUpdates {
		_ = conn.SetReadDeadline(time.Now().Add(time.Duration(idleTimeoutMs) * time.Millisecond))
		pkt, err := readTPKT(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				break
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return out, err
		}
		if firstUpdateMs == 0 {
			firstUpdateMs = time.Since(started).Milliseconds()
		}
		stats, err := applyBitmapUpdatePacket(screenshot, pkt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scene %s bitmap decode warning: %v\n", scene.Name, err)
			break
		}
		out.Updates++
		out.BitmapPayloadBytes += int64(stats.Bytes)
		out.BitmapRectangles += stats.Rectangles
		out.BitmapPixels += int64(stats.Pixels)
		summary.BitmapUpdates++
		summary.BitmapPayloadBytes += int64(stats.Bytes)
		summary.BitmapRectangles += stats.Rectangles
		summary.BitmapPixels += int64(stats.Pixels)
	}
	_ = conn.SetReadDeadline(time.Time{})
	out.DurationMs = time.Since(started).Milliseconds()
	out.FirstUpdateMs = firstUpdateMs
	out.BytesRead = summary.BytesRead - beforeBytes
	out.ThroughputMbps = mbps(out.BytesRead, out.DurationMs)
	if err := writePNG(shot, screenshot); err != nil {
		return out, err
	}
	return out, nil
}

func readDemandActiveOrSkipLicense(conn net.Conn) ([]byte, error) {
	for i := 0; i < 3; i++ {
		pkt, err := readTPKT(conn)
		if err != nil {
			return nil, err
		}
		if isLicensePDU(pkt) {
			fmt.Printf("License: %x\n", pkt)
			continue
		}
		return pkt, nil
	}
	return nil, fmt.Errorf("Demand Active not received after license PDUs")
}

func isLicensePDU(pkt []byte) bool {
	// TPKT has already been removed. Look for X.224 Data + MCS SendDataIndication
	// carrying a security header with SEC_LICENSE_PKT (0x0080).
	for i := 0; i+4 <= len(pkt); i++ {
		if binary.LittleEndian.Uint16(pkt[i:i+2])&0x0080 != 0 {
			return true
		}
	}
	return false
}

func readTPKT(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint16(header[2:4]))
	payload := make([]byte, length-4)
	_, err := io.ReadFull(r, payload)
	if err == nil {
		summary.PacketsRead++
		summary.BytesRead += int64(length)
		tracePacket("server", payload)
	}
	return payload, err
}

func writeTPKT(w io.Writer, payload []byte) error {
	header := []byte{3, 0, 0, 0}
	binary.BigEndian.PutUint16(header[2:4], uint16(4+len(payload)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	if err == nil {
		summary.PacketsWritten++
		summary.BytesWritten += int64(4 + len(payload))
		tracePacket("client", payload)
	}
	return err
}

func tracePacket(direction string, payload []byte) {
	v := traceOut.Load()
	if v == nil {
		return
	}
	dir, ok := v.(string)
	if !ok || dir == "" {
		return
	}
	name := filepath.Join(dir, fmt.Sprintf("%03d-%s.hex", nextTraceID(), direction))
	_ = os.WriteFile(name, []byte(hex.Dump(payload)), 0o644)
}

var traceCounter uint64

func nextTraceID() uint64 { return atomic.AddUint64(&traceCounter, 1) }

func parseSelectedProtocol(resp []byte) uint32 {
	if len(resp) >= 15 && resp[7] == 0x02 {
		return binary.LittleEndian.Uint32(resp[11:15])
	}
	return 0
}

func sendX224ConnectionRequest(conn net.Conn, nla bool) error {
	neg := make([]byte, 8)
	neg[0] = 0x01 // RDP_NEG_REQ
	binary.LittleEndian.PutUint16(neg[2:4], 8)
	protocols := uint32(0x00000001) // SSL requested
	if nla {
		protocols |= 0x00000002 // HYBRID/NLA requested
	}
	binary.LittleEndian.PutUint32(neg[4:8], protocols)
	userData := append([]byte("Cookie: mstshash=probe\r\n"), neg...)
	li := byte(6 + len(userData))
	x224 := []byte{li, 0xe0, 0, 0, 0, 1, 0}
	x224 = append(x224, userData...)
	return writeTPKT(conn, x224)
}

func sendMCSConnectInitial(conn net.Conn) error {
	return writeTPKT(conn, []byte{0x02, 0xf0, 0x80, 0x7f, 0x65, 0x00})
}

func sendMCSDomainPDU(conn net.Conn, application int, body []byte) error {
	mcs := append([]byte{byte(application << 2)}, body...)
	return writeTPKT(conn, append([]byte{0x02, 0xf0, 0x80}, mcs...))
}

func encodePERInteger16(value, minimum uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, value-minimum)
	return buf
}

func buildMCSSendDataRequest(initiator, channelID uint16, data []byte) []byte {
	body := encodePERInteger16(initiator, 1001)
	body = append(body, encodePERInteger16(channelID, 0)...)
	body = append(body, 0x70)
	body = append(body, encodePERLength(len(data))...)
	body = append(body, data...)
	return body
}

func encodePERLength(length int) []byte {
	if length > 0x7f {
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(length)|0x8000)
		return buf
	}
	return []byte{byte(length)}
}

func readAndPrint(conn net.Conn, label string) []byte {
	pkt, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	if dumpPackets.Load() {
		fmt.Printf("%s: %x\n", label, pkt)
	} else {
		fmt.Printf("%s: %d bytes\n", label, len(pkt))
	}
	return pkt
}

type bitmapStats struct {
	Rectangles int
	Pixels     int
	Bytes      int
}

func applyBitmapUpdatePacket(dst *image.RGBA, pkt []byte) (bitmapStats, error) {
	if len(pkt) < 4 || pkt[0] != 0x02 || pkt[1] != 0xf0 || pkt[2] != 0x80 {
		return bitmapStats{}, fmt.Errorf("not an X.224 Data TPDU")
	}
	mcs := pkt[3:]
	if len(mcs) < 2 || int(mcs[0]>>2) != 26 {
		return bitmapStats{}, fmt.Errorf("not an MCS SendDataIndication")
	}
	body := mcs[1:]
	if len(body) < 6 || body[4] != 0x70 {
		return bitmapStats{}, fmt.Errorf("short SendDataIndication")
	}
	length, consumed, err := readPERLengthBytes(body[5:])
	if err != nil {
		return bitmapStats{}, err
	}
	dataStart := 5 + consumed
	if dataStart+length > len(body) {
		return bitmapStats{}, fmt.Errorf("SendDataIndication length %d exceeds available %d", length, len(body)-dataStart)
	}
	share := body[dataStart : dataStart+length]
	if len(share) < 18 {
		return bitmapStats{}, fmt.Errorf("short Share Data PDU")
	}
	total := int(binary.LittleEndian.Uint16(share[0:2]))
	if total > len(share) {
		return bitmapStats{}, fmt.Errorf("share length %d exceeds available %d", total, len(share))
	}
	if binary.LittleEndian.Uint16(share[2:4]) != 0x0017 {
		return bitmapStats{}, fmt.Errorf("not Share Data")
	}
	payload := share[6:total]
	if len(payload) < 12 || payload[8] != 0x02 {
		return bitmapStats{}, fmt.Errorf("not a bitmap Update PDU")
	}
	return applyBitmapUpdate(dst, payload[12:])
}

func readPERLengthBytes(data []byte) (length, consumed int, err error) {
	if len(data) == 0 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	if data[0]&0x80 == 0 {
		return int(data[0]), 1, nil
	}
	if len(data) < 2 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	return (int(data[0]&0x7f) << 8) | int(data[1]), 2, nil
}

func applyBitmapUpdate(dst *image.RGBA, payload []byte) (bitmapStats, error) {
	if len(payload) < 4 {
		return bitmapStats{}, fmt.Errorf("short bitmap update")
	}
	if binary.LittleEndian.Uint16(payload[0:2]) != 0x0001 {
		return bitmapStats{}, fmt.Errorf("not a bitmap update")
	}
	rects := int(binary.LittleEndian.Uint16(payload[2:4]))
	stats := bitmapStats{Rectangles: rects}
	r := bytes.NewReader(payload[4:])
	for i := 0; i < rects; i++ {
		var left, top, right, bottom, width, height, bpp, flags, dataLen uint16
		for _, v := range []*uint16{&left, &top, &right, &bottom, &width, &height, &bpp, &flags, &dataLen} {
			if err := binary.Read(r, binary.LittleEndian, v); err != nil {
				return bitmapStats{}, err
			}
		}
		data := make([]byte, dataLen)
		stats.Bytes += int(dataLen)
		stats.Pixels += int(width) * int(height)
		if _, err := io.ReadFull(r, data); err != nil {
			return bitmapStats{}, err
		}
		if bpp != 32 || flags != 0 {
			return bitmapStats{}, fmt.Errorf("unsupported bitmap rect bpp=%d flags=0x%04x", bpp, flags)
		}
		if int(width)*int(height)*4 > len(data) {
			return bitmapStats{}, fmt.Errorf("short bitmap rect data")
		}
		_ = right
		_ = bottom
		for y := 0; y < int(height); y++ {
			for x := 0; x < int(width); x++ {
				si := (y*int(width) + x) * 4
				dst.SetRGBA(int(left)+x, int(top)+y, color.RGBA{R: data[si+2], G: data[si+1], B: data[si], A: data[si+3]})
			}
		}
	}
	return stats, nil
}

func mbps(bytes int64, durationMs int64) float64 {
	if durationMs <= 0 {
		return 0
	}
	return (float64(bytes) * 8.0) / (float64(durationMs) / 1000.0) / 1_000_000.0
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func buildSlowPathInputEvent(messageType, flags, codeOrX, y uint16) []byte {
	out := appendLE32(nil, 0)
	out = appendLE16(out, messageType)
	out = appendLE16(out, flags)
	out = appendLE16(out, codeOrX)
	out = appendLE16(out, y)
	return out
}

func buildSlowPathInputPDU(events ...[]byte) []byte {
	out := appendLE16(nil, uint16(len(events)))
	out = appendLE16(out, 0)
	for _, event := range events {
		out = append(out, event...)
	}
	return out
}

func sendShareData(conn net.Conn, pduType2 byte, payload []byte) error {
	pdu := buildShareDataPDU(pduType2, payload)
	return sendMCSDomainPDU(conn, 25, buildMCSSendDataRequest(1001, 1003, pdu))
}

func buildClientInfoPayload(username, password, domain string) []byte {
	fields := [][]byte{
		encodeClientInfoString(domain),
		encodeClientInfoString(username),
		encodeClientInfoString(password),
		encodeClientInfoString(""),
		encodeClientInfoString(""),
	}
	out := make([]byte, 18)
	binary.LittleEndian.PutUint32(out[4:8], 0x00000010) // INFO_UNICODE
	for i, field := range fields {
		binary.LittleEndian.PutUint16(out[8+i*2:10+i*2], uint16(len(field)))
	}
	for _, field := range fields {
		out = append(out, field...)
	}
	return out
}

func encodeClientInfoString(s string) []byte {
	runes := utf16.Encode([]rune(s + "\x00"))
	out := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], r)
	}
	return out
}

func buildShareDataPDU(pduType2 byte, payload []byte) []byte {
	totalLength := 18 + len(payload)
	out := appendLE16(nil, uint16(totalLength))
	out = appendLE16(out, 0x0017)
	out = appendLE16(out, 1001)
	out = appendLE32(out, 0x000103ea)
	out = append(out, 0, 1)
	out = appendLE16(out, uint16(4+len(payload)))
	out = append(out, pduType2, 0)
	out = appendLE16(out, 0)
	out = append(out, payload...)
	return out
}

func synchronizePayload() []byte {
	out := appendLE16(nil, 1)
	out = appendLE16(out, 1002)
	return out
}

func controlPayload(action uint16) []byte {
	out := appendLE16(nil, action)
	out = appendLE16(out, 0)
	out = appendLE32(out, 0)
	return out
}

func buildConfirmActivePDU(shareID uint32, userID uint16) []byte {
	source := []byte("PROBE")
	cap := capabilitySet(0x0001, generalCapability())
	combinedCapsLen := 4 + len(cap)
	totalLength := 6 + 4 + 2 + 2 + 2 + len(source) + combinedCapsLen
	pdu := appendLE16(nil, uint16(totalLength))
	pdu = appendLE16(pdu, 0x0013)
	pdu = appendLE16(pdu, userID)
	pdu = appendLE32(pdu, shareID)
	pdu = appendLE16(pdu, 1002)
	pdu = appendLE16(pdu, uint16(len(source)))
	pdu = appendLE16(pdu, uint16(combinedCapsLen))
	pdu = append(pdu, source...)
	pdu = appendLE16(pdu, 1)
	pdu = appendLE16(pdu, 0)
	pdu = append(pdu, cap...)
	return pdu
}

func capabilitySet(capType uint16, payload []byte) []byte {
	out := appendLE16(nil, capType)
	out = appendLE16(out, uint16(4+len(payload)))
	out = append(out, payload...)
	return out
}

func generalCapability() []byte {
	out := make([]byte, 0, 22)
	for _, v := range []uint16{1, 3, 0x0200, 0, 0, 0, 0, 0, 0, 0, 0} {
		out = appendLE16(out, v)
	}
	return out
}

func appendLE16(dst []byte, v uint16) []byte {
	return append(dst, byte(v), byte(v>>8))
}

func appendLE32(dst []byte, v uint32) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func performClientCredSSP(conn net.Conn, tlsConn *tls.Conn, username, password, domain string) error {
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("NLA: server certificate unavailable")
	}
	pubKey := state.PeerCertificates[0].RawSubjectPublicKeyInfo
	ntlm := rdpauth.NewNTLMv2(domain, username, password)
	clientNonce := make([]byte, 32)
	if _, err := rand.Read(clientNonce); err != nil {
		return err
	}
	if _, err := conn.Write(rdpauth.EncodeTSRequestWithNonce([][]byte{ntlm.GetNegotiateMessage()}, nil, nil, clientNonce)); err != nil {
		return fmt.Errorf("NLA: send negotiate: %w", err)
	}
	challengeBytes, err := readCredSSPMessage(conn)
	if err != nil {
		return fmt.Errorf("NLA: read challenge: %w", err)
	}
	challengeReq, err := rdpauth.DecodeTSRequest(challengeBytes)
	if err != nil {
		return fmt.Errorf("NLA: decode challenge: %w", err)
	}
	if len(challengeReq.NegoTokens) == 0 {
		return fmt.Errorf("NLA: missing challenge token")
	}
	authMsg, sec := ntlm.GetAuthenticateMessage(challengeReq.NegoTokens[0].Data)
	if authMsg == nil || sec == nil {
		return fmt.Errorf("NLA: build authenticate message")
	}
	pubKeyAuth := sec.GssEncrypt(rdpauth.ComputeClientPubKeyAuth(challengeReq.Version, pubKey, clientNonce))
	if _, err := conn.Write(rdpauth.EncodeTSRequestWithNonce([][]byte{authMsg}, nil, pubKeyAuth, clientNonce)); err != nil {
		return fmt.Errorf("NLA: send authenticate: %w", err)
	}
	serverPubKeyBytes, err := readCredSSPMessage(conn)
	if err != nil {
		return fmt.Errorf("NLA: read server pubKeyAuth: %w", err)
	}
	serverPubKeyReq, err := rdpauth.DecodeTSRequest(serverPubKeyBytes)
	if err != nil {
		return fmt.Errorf("NLA: decode server pubKeyAuth: %w", err)
	}
	serverPubKeyAuth := sec.GssDecrypt(serverPubKeyReq.PubKeyAuth)
	if !bytes.Equal(serverPubKeyAuth, rdpauth.ComputeServerPubKeyAuth(serverPubKeyReq.Version, pubKey, clientNonce)) {
		return fmt.Errorf("NLA: server pubKeyAuth mismatch")
	}
	domainBytes, userBytes, passBytes := ntlm.GetCredSSPCredentials()
	creds := rdpauth.EncodeCredentials(domainBytes, userBytes, passBytes)
	if _, err := conn.Write(rdpauth.EncodeTSRequest(nil, sec.GssEncrypt(creds), nil)); err != nil {
		return fmt.Errorf("NLA: send credentials: %w", err)
	}
	return nil
}

func readCredSSPMessage(r io.Reader) ([]byte, error) {
	first := make([]byte, 2)
	if _, err := io.ReadFull(r, first); err != nil {
		return nil, err
	}
	if first[0] != 0x30 {
		return nil, fmt.Errorf("unexpected CredSSP tag 0x%02x", first[0])
	}
	length, lenBytes, err := parseDERLength(first[1], r)
	if err != nil {
		return nil, err
	}
	if length > 64*1024 {
		return nil, fmt.Errorf("CredSSP message too large: %d", length)
	}
	out := append([]byte{}, first...)
	out = append(out, lenBytes...)
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return append(out, body...), nil
}

func parseDERLength(first byte, r io.Reader) (int, []byte, error) {
	if first < 0x80 {
		return int(first), nil, nil
	}
	n := int(first & 0x7f)
	if n == 0 || n > 4 {
		return 0, nil, fmt.Errorf("unsupported DER length byte 0x%02x", first)
	}
	extra := make([]byte, n)
	if _, err := io.ReadFull(r, extra); err != nil {
		return 0, nil, err
	}
	length := 0
	for _, b := range extra {
		length = (length << 8) | int(b)
	}
	return length, extra, nil
}
