package rdpserver

import (
	"net"
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestLatestAvailableFrameCoalescesBacklog(t *testing.T) {
	frameCh := make(chan frame.Frame, 3)
	first := frame.Frame{Width: 1, Height: 1, Timestamp: time.Unix(1, 0), Data: []byte{1}}
	second := frame.Frame{Width: 2, Height: 1, Timestamp: time.Unix(2, 0), Data: []byte{2}}
	third := frame.Frame{Width: 3, Height: 1, Timestamp: time.Unix(3, 0), Data: []byte{3}}
	frameCh <- second
	frameCh <- third
	latest := latestAvailableFrame(frameCh, first)
	if latest.Width != 3 || latest.Timestamp != third.Timestamp || len(frameCh) != 0 {
		t.Fatalf("expected latest queued frame, got width=%d ts=%v remaining=%d", latest.Width, latest.Timestamp, len(frameCh))
	}
}

func TestLatestAvailableFrameKeepsCurrentWhenNoBacklog(t *testing.T) {
	frameCh := make(chan frame.Frame)
	current := frame.Frame{Width: 4, Height: 1, Timestamp: time.Unix(4, 0), Data: []byte{4}}
	latest := latestAvailableFrame(frameCh, current)
	if latest.Width != current.Width || latest.Timestamp != current.Timestamp {
		t.Fatalf("expected current frame, got %#v", latest)
	}
}

func TestBuildAndParseShareDataPDU(t *testing.T) {
	wire := buildShareDataPDU(pduType2Synchronize, buildSynchronizePayload())
	share, err := parseShareControlPDU(wire)
	if err != nil {
		t.Fatal(err)
	}
	data, err := parseShareDataPDU(share)
	if err != nil {
		t.Fatal(err)
	}
	if data.PDUType2 != pduType2Synchronize || data.ShareID != defaultShareID || len(data.Payload) != 4 {
		t.Fatalf("unexpected share data: %#v", data)
	}
}

type benchmarkWriteConn struct{ n int }

func (c *benchmarkWriteConn) Read(_ []byte) (int, error)         { return 0, nil }
func (c *benchmarkWriteConn) Write(p []byte) (int, error)        { c.n += len(p); return len(p), nil }
func (c *benchmarkWriteConn) Close() error                       { return nil }
func (c *benchmarkWriteConn) LocalAddr() net.Addr                { return nil }
func (c *benchmarkWriteConn) RemoteAddr() net.Addr               { return nil }
func (c *benchmarkWriteConn) SetDeadline(_ time.Time) error      { return nil }
func (c *benchmarkWriteConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *benchmarkWriteConn) SetWriteDeadline(_ time.Time) error { return nil }

func BenchmarkBuildMCSSendDataIndication_4KiB(b *testing.B) {
	payload := make([]byte, 4096)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		body := buildMCSSendDataIndication(serverChannelID, globalChannelID, payload)
		if len(body) <= len(payload) {
			b.Fatalf("buildMCSSendDataIndication len=%d", len(body))
		}
	}
}

func BenchmarkBuildMCSSendDataRequest_4KiB(b *testing.B) {
	payload := make([]byte, 4096)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		body := buildMCSSendDataRequest(defaultMCSUserID, globalChannelID, payload)
		if len(body) <= len(payload) {
			b.Fatalf("buildMCSSendDataRequest len=%d", len(body))
		}
	}
}

func BenchmarkWriteMCSDomainPDU_4KiB(b *testing.B) {
	body := make([]byte, 4096)
	conn := &benchmarkWriteConn{}
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := writeMCSDomainPDU(conn, mcsSendDataIndicationApp, body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteShareDataPDU_4KiB(b *testing.B) {
	payload := make([]byte, 4096)
	conn := &benchmarkWriteConn{}
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := writeShareDataPDU(conn, pduType2Update, payload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildShareDataPDU_4KiB(b *testing.B) {
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i)
	}
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		wire := buildShareDataPDU(pduType2Update, payload)
		if len(wire) != len(payload)+18 {
			b.Fatalf("buildShareDataPDU len=%d", len(wire))
		}
	}
}

func TestControlAndFontPayloads(t *testing.T) {
	ctrl := buildControlPayload(controlActionGrantedControl)
	action, err := parseControlAction(ctrl)
	if err != nil {
		t.Fatal(err)
	}
	if action != controlActionGrantedControl {
		t.Fatalf("unexpected action %d", action)
	}
	font := buildFontMapPayload()
	if len(font) != 8 || font[4] != 3 {
		t.Fatalf("unexpected font map payload: %x", font)
	}
}
