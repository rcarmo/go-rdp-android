package rdpserver

import "testing"

func TestParseX224Data(t *testing.T) {
	got, err := parseX224Data([]byte{0x02, x224TypeData, 0x80, 0x7f})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 0x7f {
		t.Fatalf("unexpected payload: %x", got)
	}
}

func TestParseMCSConnectInitialAppTag(t *testing.T) {
	// BER application tag 101 encoded as [APPLICATION 101] constructed:
	// 0x7f 0x65, length 0.
	info, err := parseMCSConnectInitial([]byte{0x7f, 0x65, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	if info.ApplicationTag != mcsConnectInitialAppTag || info.PayloadLength != 0 {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestReadBERLength(t *testing.T) {
	buf := byteReader{data: []byte{0x82, 0x01, 0x00}}
	got, err := readBERLength(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != 256 {
		t.Fatalf("expected 256, got %d", got)
	}
}

type byteReader struct{ data []byte }

func (r *byteReader) ReadByte() (byte, error) {
	if len(r.data) == 0 {
		return 0, errEOF{}
	}
	b := r.data[0]
	r.data = r.data[1:]
	return b, nil
}

type errEOF struct{}

func (errEOF) Error() string { return "eof" }
