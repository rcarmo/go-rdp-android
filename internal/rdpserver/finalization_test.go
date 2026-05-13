package rdpserver

import (
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
