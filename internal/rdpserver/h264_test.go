package rdpserver

import "testing"

func TestH264EnabledFromEnv(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_DISABLE_H264", "")
	if !h264EnabledFromEnv() {
		t.Fatal("h264EnabledFromEnv() = false, want true by default")
	}
	for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("GO_RDP_ANDROID_DISABLE_H264", value)
			if h264EnabledFromEnv() {
				t.Fatalf("h264EnabledFromEnv() = true for %q, want false", value)
			}
		})
	}
	for _, value := range []string{"0", "false", "off", ""} {
		t.Run("enabled_"+value, func(t *testing.T) {
			t.Setenv("GO_RDP_ANDROID_DISABLE_H264", value)
			if !h264EnabledFromEnv() {
				t.Fatalf("h264EnabledFromEnv() = false for %q, want true", value)
			}
		})
	}
}

func TestH264NormalizeAnnexB(t *testing.T) {
	annexB, ok := h264NormalizeAnnexB([]byte{0, 0, 0, 1, 0x65})
	if !ok || string(annexB) != string([]byte{0, 0, 0, 1, 0x65}) {
		t.Fatalf("annexB normalize = %x ok=%t", annexB, ok)
	}
	lengthPrefixed, ok := h264NormalizeAnnexB([]byte{0, 0, 0, 2, 0x67, 0x01, 0, 0, 0, 1, 0x68})
	want := []byte{0, 0, 0, 1, 0x67, 0x01, 0, 0, 0, 1, 0x68}
	if !ok || string(lengthPrefixed) != string(want) {
		t.Fatalf("length-prefixed normalize = %x ok=%t, want %x", lengthPrefixed, ok, want)
	}
	if _, ok := h264NormalizeAnnexB([]byte{0, 0, 0, 4, 0x65}); ok {
		t.Fatal("invalid length-prefixed access unit normalized successfully")
	}
}

func TestH264StreamStatePrepareForWire(t *testing.T) {
	var state h264StreamState
	if _, ok := state.prepareForWire(h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x41}}); ok {
		t.Fatal("non-keyframe before keyframe should not be ready")
	}
	if _, ok := state.prepareForWire(h264AccessUnit{PresentationTimeUS: 2, CodecConfig: true, Data: []byte{0, 0, 0, 1, 0x67, 0, 0, 0, 1, 0x68}}); ok {
		t.Fatal("codec config only should not be ready")
	}
	unit, ok := state.prepareForWire(h264AccessUnit{PresentationTimeUS: 3, KeyFrame: true, Data: []byte{0, 0, 0, 1, 0x65}})
	if !ok {
		t.Fatal("keyframe after config should be ready")
	}
	if got := string(unit.Data); got != string([]byte{0, 0, 0, 1, 0x67, 0, 0, 0, 1, 0x68, 0, 0, 0, 1, 0x65}) {
		t.Fatalf("combined data = %x, want config+keyframe", unit.Data)
	}
	if _, ok := state.prepareForWire(h264AccessUnit{PresentationTimeUS: 4, Data: []byte{0, 0, 0, 1, 0x41}}); !ok {
		t.Fatal("non-keyframe after keyframe should be ready")
	}
}

func TestValidateH264AccessUnit(t *testing.T) {
	valid := h264AccessUnit{PresentationTimeUS: 1, KeyFrame: true, Data: []byte{0, 0, 1, 0x65}}
	if err := validateH264AccessUnit(valid); err != nil {
		t.Fatalf("validateH264AccessUnit(valid) error = %v", err)
	}

	tests := []struct {
		name string
		unit h264AccessUnit
	}{
		{name: "empty", unit: h264AccessUnit{Data: nil}},
		{name: "oversize", unit: h264AccessUnit{Data: make([]byte, h264MaxAccessUnitLen+1)}},
		{name: "negative pts", unit: h264AccessUnit{PresentationTimeUS: -1, Data: []byte{1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateH264AccessUnit(tt.unit); err == nil {
				t.Fatal("validateH264AccessUnit() error = nil, want error")
			}
		})
	}
}

func TestValidateH264AccessUnitBatch(t *testing.T) {
	valid := h264AccessUnit{PresentationTimeUS: 1, Data: []byte{1}}
	if err := validateH264AccessUnitBatch([]h264AccessUnit{valid}); err != nil {
		t.Fatalf("validateH264AccessUnitBatch(valid) error = %v", err)
	}
	if err := validateH264AccessUnitBatch(nil); err == nil {
		t.Fatal("validateH264AccessUnitBatch(nil) error = nil, want error")
	}
	tooMany := make([]h264AccessUnit, h264MaxAccessUnits+1)
	for i := range tooMany {
		tooMany[i] = valid
	}
	if err := validateH264AccessUnitBatch(tooMany); err == nil {
		t.Fatal("validateH264AccessUnitBatch(tooMany) error = nil, want error")
	}
	if err := validateH264AccessUnitBatch([]h264AccessUnit{{Data: nil}}); err == nil {
		t.Fatal("validateH264AccessUnitBatch(invalid unit) error = nil, want error")
	}
}
