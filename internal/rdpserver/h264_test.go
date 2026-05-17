package rdpserver

import "testing"

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
