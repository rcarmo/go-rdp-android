package rdpserver

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

const (
	tpktVersion = 3

	x224TypeConnectionRequest = 0xe0
	x224TypeConnectionConfirm = 0xd0

	rdpNegReq  = 0x01
	rdpNegResp = 0x02

	protocolRDP    = 0x00000000
	protocolSSL    = 0x00000001
	protocolHybrid = 0x00000002
)

var errFastPathPDU = errors.New("fast-path packet ignored")

type transportPDU struct {
	FastPath bool
	Header   byte
	Payload  []byte
}

// HandshakeInfo captures the initial client negotiation request.
type HandshakeInfo struct {
	Cookie             string
	RequestedProtocols uint32
	SelectedProtocol   uint32
	TLSPublicKey       []byte
}

func performInitialHandshake(conn net.Conn) (*HandshakeInfo, net.Conn, error) {
	payload, err := readTPKT(conn)
	if err != nil {
		return nil, nil, fmt.Errorf("read tpkt: %w", err)
	}

	userData, srcRef, err := parseX224ConnectionRequest(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("parse x224 connection request: %w", err)
	}

	info := parseNegotiationUserData(userData)
	if info.RequestedProtocols&protocolHybrid != 0 {
		info.SelectedProtocol = protocolHybrid
	} else if info.RequestedProtocols&protocolSSL != 0 {
		info.SelectedProtocol = protocolSSL
	} else {
		info.SelectedProtocol = protocolRDP
	}

	if err := writeConnectionConfirm(conn, srcRef, info.SelectedProtocol); err != nil {
		return nil, nil, fmt.Errorf("write x224 connection confirm: %w", err)
	}
	if info.SelectedProtocol == protocolSSL || info.SelectedProtocol == protocolHybrid {
		cfg, err := defaultTLSConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("tls config: %w", err)
		}
		tlsConn := tls.Server(conn, cfg)
		if err := tlsConn.Handshake(); err != nil {
			return nil, nil, fmt.Errorf("tls handshake: %w", err)
		}
		info.TLSPublicKey = tlsPublicKeyFromConfig(cfg)
		return &info, tlsConn, nil
	}

	return &info, conn, nil
}

func readTPKT(r io.Reader) ([]byte, error) {
	pdu, err := readTransportPDU(r)
	if err != nil {
		return nil, err
	}
	if pdu.FastPath {
		tracef("fastpath_ignore", "first=0x%02x length=%d", pdu.Header, 2+len(pdu.Payload))
		return nil, errFastPathPDU
	}
	return pdu.Payload, nil
}

func readTransportPDU(r io.Reader) (*transportPDU, error) {
	first := make([]byte, 1)
	if _, err := io.ReadFull(r, first); err != nil {
		return nil, err
	}
	if first[0] != tpktVersion {
		payload, err := readFastPathTransportPayload(r)
		if err != nil {
			return nil, err
		}
		return &transportPDU{FastPath: true, Header: first[0], Payload: payload}, nil
	}

	headerRest := make([]byte, 3)
	if _, err := io.ReadFull(r, headerRest); err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint16(headerRest[1:3]))
	if length < 4 {
		return nil, fmt.Errorf("invalid TPKT length %d", length)
	}
	payload := make([]byte, length-4)
	_, err := io.ReadFull(r, payload)
	if err == nil {
		tracef("tpkt_read", "payload_len=%d", len(payload))
	}
	return &transportPDU{Payload: payload}, err
}

func readFastPathTransportPayload(r io.Reader) ([]byte, error) {
	lengthBytes := make([]byte, 1)
	if _, err := io.ReadFull(r, lengthBytes); err != nil {
		return nil, err
	}
	length := int(lengthBytes[0])
	headerLen := 2
	if lengthBytes[0]&0x80 != 0 {
		second := make([]byte, 1)
		if _, err := io.ReadFull(r, second); err != nil {
			return nil, err
		}
		length = (int(lengthBytes[0]&0x7f) << 8) | int(second[0])
		headerLen = 3
	}
	if length < headerLen {
		return nil, fmt.Errorf("invalid Fast-Path length %d", length)
	}
	payload := make([]byte, length-headerLen)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

func writeTPKT(w io.Writer, payload []byte) error {
	length := 4 + len(payload)
	if length > 0xffff {
		return fmt.Errorf("TPKT payload too large: %d", len(payload))
	}
	header := []byte{tpktVersion, 0, 0, 0}
	binary.BigEndian.PutUint16(header[2:4], uint16(length))
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	if err == nil {
		tracef("tpkt_write", "payload_len=%d", len(payload))
	}
	return err
}

func parseX224ConnectionRequest(payload []byte) ([]byte, uint16, error) {
	if len(payload) < 7 {
		return nil, 0, errors.New("short X.224 connection request")
	}
	li := int(payload[0])
	if li+1 > len(payload) || li < 6 {
		return nil, 0, fmt.Errorf("invalid X.224 length indicator %d", li)
	}
	if payload[1] != x224TypeConnectionRequest {
		return nil, 0, fmt.Errorf("unexpected X.224 type 0x%02x", payload[1])
	}
	srcRef := binary.BigEndian.Uint16(payload[4:6])
	return payload[7 : li+1], srcRef, nil
}

func parseNegotiationUserData(userData []byte) HandshakeInfo {
	info := HandshakeInfo{}
	idx := 0
	if end := strings.Index(string(userData), "\r\n"); end >= 0 {
		line := string(userData[:end])
		if strings.HasPrefix(line, "Cookie:") {
			info.Cookie = strings.TrimSpace(strings.TrimPrefix(line, "Cookie:"))
			idx = end + 2
		}
	}
	if len(userData)-idx >= 8 && userData[idx] == rdpNegReq {
		info.RequestedProtocols = binary.LittleEndian.Uint32(userData[idx+4 : idx+8])
	}
	return info
}

func writeConnectionConfirm(conn net.Conn, dstRef uint16, selectedProtocol uint32) error {
	neg := make([]byte, 8)
	neg[0] = rdpNegResp
	neg[1] = 0
	binary.LittleEndian.PutUint16(neg[2:4], uint16(len(neg)))
	binary.LittleEndian.PutUint32(neg[4:8], selectedProtocol)

	li := byte(6 + len(neg))
	x224 := []byte{li, x224TypeConnectionConfirm, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(x224[2:4], dstRef)
	x224 = append(x224, neg...)
	tracef("x224_confirm", "dst_ref=%d selected_protocol=0x%08x", dstRef, selectedProtocol)
	return writeTPKT(conn, x224)
}
