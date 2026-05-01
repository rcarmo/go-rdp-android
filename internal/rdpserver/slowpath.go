package rdpserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	mcsSendDataRequestApp    = 25
	mcsSendDataIndicationApp = 26

	secExchangePacket = 0x0001
	secInfoPacket     = 0x0040
)

type sendDataRequest struct {
	Initiator uint16
	ChannelID uint16
	Data      []byte
}

type securityPDU struct {
	Flags     uint16
	FlagsHi   uint16
	Payload   []byte
	KindLabel string
}

func parseMCSSendDataRequest(body []byte) (*sendDataRequest, error) {
	if len(body) < 6 {
		return nil, fmt.Errorf("short SendDataRequest")
	}
	r := bytes.NewReader(body)
	var initiatorWire, channelID uint16
	if err := binary.Read(r, binary.BigEndian, &initiatorWire); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &channelID); err != nil {
		return nil, err
	}
	magic, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if magic != 0x70 {
		return nil, fmt.Errorf("unexpected SendDataRequest segmentation byte 0x%02x", magic)
	}
	length, err := readPERLength(r)
	if err != nil {
		return nil, err
	}
	if length > r.Len() {
		return nil, fmt.Errorf("SendDataRequest length %d exceeds available %d", length, r.Len())
	}
	data := make([]byte, length)
	if _, err := r.Read(data); err != nil {
		return nil, err
	}
	return &sendDataRequest{Initiator: initiatorWire + defaultMCSUserID, ChannelID: channelID, Data: data}, nil
}

func parseSecurityPDU(data []byte) (*securityPDU, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("short security PDU")
	}
	pdu := &securityPDU{
		Flags:   binary.LittleEndian.Uint16(data[0:2]),
		FlagsHi: binary.LittleEndian.Uint16(data[2:4]),
		Payload: data[4:],
	}
	switch {
	case pdu.Flags&secExchangePacket != 0:
		pdu.KindLabel = "security-exchange"
	case pdu.Flags&secInfoPacket != 0:
		pdu.KindLabel = "client-info"
	default:
		pdu.KindLabel = "unknown"
	}
	return pdu, nil
}

func buildMCSSendDataRequest(initiator, channelID uint16, data []byte) []byte {
	body := encodePERInteger16(initiator, defaultMCSUserID)
	body = append(body, encodePERInteger16(channelID, 0)...)
	body = append(body, 0x70)
	body = append(body, encodePERLength(len(data))...)
	body = append(body, data...)
	return body
}

func readPERLength(r *bytes.Reader) (int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if b&0x80 == 0 {
		return int(b), nil
	}
	next, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	return (int(b&0x7f) << 8) | int(next), nil
}

func encodePERLength(length int) []byte {
	if length > 0x7f {
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(length)|0x8000)
		return buf
	}
	return []byte{byte(length)}
}
