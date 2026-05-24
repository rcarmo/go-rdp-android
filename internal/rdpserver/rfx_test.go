package rdpserver

import "testing"

func TestBuildRFXSurfaceBitsCommand(t *testing.T) {
	payload := []byte{0xc0, 0xcc, 0x0c, 0x00, 0x00, 0x00, 0xca, 0xca, 0xcc, 0xca, 0x00, 0x01}
	cmd, ok := buildRFXSurfaceBitsCommand(64, 64, 4, payload)
	if !ok {
		t.Fatal("buildRFXSurfaceBitsCommand ok = false")
	}
	cmdType, codecID, width, height, bitmapLen, ok := parseSurfaceBitsCommandHeaderForTest(cmd)
	if !ok {
		t.Fatal("parseSurfaceBitsCommandHeaderForTest ok = false")
	}
	if cmdType != surfaceCmdSetSurfaceBits || codecID != 4 || width != 64 || height != 64 || bitmapLen != uint32(len(payload)) {
		t.Fatalf("header cmd=0x%04x codec=%d width=%d height=%d len=%d", cmdType, codecID, width, height, bitmapLen)
	}
	if got := cmd[surfaceBitsHeaderLen:]; string(got) != string(payload) {
		t.Fatalf("payload mismatch got %x want %x", got, payload)
	}
}

func TestBuildRFXSurfaceBitsCommandRejectsInvalid(t *testing.T) {
	if cmd, ok := buildRFXSurfaceBitsCommand(64, 64, 4, nil); ok || cmd != nil {
		t.Fatalf("empty payload cmd len=%d ok=%t", len(cmd), ok)
	}
	if cmd, ok := buildRFXSurfaceBitsCommand(64, 64, 0, []byte{1}); ok || cmd != nil {
		t.Fatalf("zero codec cmd len=%d ok=%t", len(cmd), ok)
	}
	if cmd, ok := buildRFXSurfaceBitsCommand(0, 64, 4, []byte{1}); ok || cmd != nil {
		t.Fatalf("zero width cmd len=%d ok=%t", len(cmd), ok)
	}
	tooLarge := make([]byte, rfxMaxEncodedPayloadLen+1)
	if cmd, ok := buildRFXSurfaceBitsCommand(64, 64, 4, tooLarge); ok || cmd != nil {
		t.Fatalf("too-large payload cmd len=%d ok=%t", len(cmd), ok)
	}
}
