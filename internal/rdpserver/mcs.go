package rdpserver

import (
	"bytes"
	"fmt"
	"io"
	"net"
)

const (
	x224TypeData = 0xf0
	mcsConnectInitialAppTag = 101
)

// MCSInfo captures the first MCS/GCC payload from the client.
type MCSInfo struct {
	ApplicationTag int
	PayloadLength  int
	UserDataLength int
}

func readMCSConnectInitial(conn net.Conn) (*MCSInfo, error) {
	payload, err := readTPKT(conn)
	if err != nil {
		return nil, fmt.Errorf("read tpkt: %w", err)
	}
	userData, err := parseX224Data(payload)
	if err != nil {
		return nil, fmt.Errorf("parse x224 data: %w", err)
	}
	return parseMCSConnectInitial(userData)
}

func parseX224Data(payload []byte) ([]byte, error) {
	// X.224 data TPDU used by RDP is typically: LI=2, DT=0xf0, EOT=0x80, followed by MCS payload.
	if len(payload) < 3 {
		return nil, fmt.Errorf("short X.224 data TPDU")
	}
	li := int(payload[0])
	if li < 2 || li+1 > len(payload) {
		return nil, fmt.Errorf("invalid X.224 data length indicator %d", li)
	}
	if payload[1] != x224TypeData {
		return nil, fmt.Errorf("unexpected X.224 data type 0x%02x", payload[1])
	}
	return payload[li+1:], nil
}

func parseMCSConnectInitial(data []byte) (*MCSInfo, error) {
	r := bytes.NewReader(data)
	appTag, payloadLen, err := readBERApplicationTag(r)
	if err != nil {
		return nil, err
	}
	if appTag != mcsConnectInitialAppTag {
		return nil, fmt.Errorf("unexpected MCS application tag %d", appTag)
	}
	if payloadLen > r.Len() {
		return nil, fmt.Errorf("MCS payload length %d exceeds available %d", payloadLen, r.Len())
	}
	return &MCSInfo{ApplicationTag: appTag, PayloadLength: payloadLen, UserDataLength: r.Len()}, nil
}

func readBERApplicationTag(r *bytes.Reader) (tag int, length int, err error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	if b&0xe0 != 0x60 {
		return 0, 0, fmt.Errorf("not a BER application tag: 0x%02x", b)
	}
	tag = int(b & 0x1f)
	if tag == 0x1f {
		tag = 0
		for {
			b, err = r.ReadByte()
			if err != nil {
				return 0, 0, err
			}
			tag = (tag << 7) | int(b&0x7f)
			if b&0x80 == 0 {
				break
			}
		}
	}
	length, err = readBERLength(r)
	return tag, length, err
}

func readBERLength(r io.ByteReader) (int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if b&0x80 == 0 {
		return int(b), nil
	}
	n := int(b & 0x7f)
	if n == 0 {
		return 0, fmt.Errorf("indefinite BER length not supported")
	}
	if n > 4 {
		return 0, fmt.Errorf("BER length too large: %d bytes", n)
	}
	length := 0
	for i := 0; i < n; i++ {
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		length = (length << 8) | int(b)
	}
	return length, nil
}
