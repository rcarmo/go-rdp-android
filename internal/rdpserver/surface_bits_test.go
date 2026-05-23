package rdpserver

import "testing"

func TestBuildSurfaceBitsCommand(t *testing.T) {
	cmd, ok := buildSurfaceBitsCommand(3, 2, 7, []byte{1, 2, 3})
	if !ok {
		t.Fatal("buildSurfaceBitsCommand() ok = false")
	}
	cmdType, codecID, width, height, bitmapLen, ok := parseSurfaceBitsCommandHeaderForTest(cmd)
	if !ok {
		t.Fatal("parseSurfaceBitsCommandHeaderForTest() ok = false")
	}
	if cmdType != surfaceCmdSetSurfaceBits || codecID != 7 || width != 3 || height != 2 || bitmapLen != 3 {
		t.Fatalf("unexpected header cmd=0x%04x codec=%d size=%dx%d len=%d", cmdType, codecID, width, height, bitmapLen)
	}
	if got := cmd[surfaceBitsHeaderLen:]; string(got) != string([]byte{1, 2, 3}) {
		t.Fatalf("payload = %v", got)
	}
}

func TestBuildSurfaceBitsCommandRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		width   int
		height  int
		codecID byte
		encoded []byte
	}{
		{name: "zero width", width: 0, height: 1, codecID: 1, encoded: []byte{1}},
		{name: "zero height", width: 1, height: 0, codecID: 1, encoded: []byte{1}},
		{name: "too wide", width: int(^uint16(0)) + 1, height: 1, codecID: 1, encoded: []byte{1}},
		{name: "zero codec", width: 1, height: 1, codecID: 0, encoded: []byte{1}},
		{name: "empty payload", width: 1, height: 1, codecID: 1, encoded: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, ok := buildSurfaceBitsCommand(tc.width, tc.height, tc.codecID, tc.encoded)
			if ok || cmd != nil {
				t.Fatalf("buildSurfaceBitsCommand invalid len=%d ok=%t", len(cmd), ok)
			}
		})
	}
}
