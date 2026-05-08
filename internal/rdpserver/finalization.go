package rdpserver

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	"github.com/rcarmo/go-rdp-android/internal/input"
)

const (
	controlActionRequestControl = 0x0001
	controlActionGrantedControl = 0x0002
	controlActionCooperate      = 0x0004
)

type shareDataPDU struct {
	ShareID            uint32
	StreamID           uint8
	UncompressedLength uint16
	PDUType2           uint8
	Payload            []byte
}

func handleShareDataPDU(conn net.Conn, share *shareControlPDU, frames frame.Source, sink input.Sink, width, height int) error {
	data, err := parseShareDataPDU(share)
	if err != nil {
		return err
	}
	tracef("share_data", "type2=0x%02x payload_len=%d", data.PDUType2, len(data.Payload))
	switch data.PDUType2 {
	case pduType2Synchronize:
		return writeShareDataPDU(conn, pduType2Synchronize, buildSynchronizePayload())
	case pduType2Control:
		action, err := parseControlAction(data.Payload)
		if err != nil {
			return err
		}
		if action == controlActionRequestControl {
			return writeShareDataPDU(conn, pduType2Control, buildControlPayload(controlActionGrantedControl))
		}
		if action == controlActionCooperate {
			return writeShareDataPDU(conn, pduType2Control, buildControlPayload(controlActionCooperate))
		}
	case pduType2FontList:
		if err := writeShareDataPDU(conn, pduType2FontMap, buildFontMapPayload()); err != nil {
			return err
		}
		return writeInitialBitmapUpdate(conn, frames, width, height)
	case pduType2Input:
		return dispatchSlowPathInput(data.Payload, sink)
	}
	return nil
}

func writeInitialBitmapUpdate(conn net.Conn, frames frame.Source, width, height int) error {
	if frames != nil {
		select {
		case fr := <-frames.Frames():
			cache := newBitmapTileCache()
			if updates, ok := buildFrameBitmapUpdatesForDesktop(fr, cache, false, width, height); ok {
				if err := writeBitmapUpdates(conn, updates); err != nil {
					return err
				}
				go streamFrameUpdates(conn, frames, cache, width, height)
				return nil
			}
		default:
		}
	}
	return writeShareDataPDU(conn, pduType2Update, buildSolidBitmapUpdate(minPositive(width, 64), minPositive(height, 64), 0xff336699))
}

func streamFrameUpdates(conn net.Conn, frames frame.Source, cache *bitmapTileCache, width, height int) {
	if cache == nil {
		cache = newBitmapTileCache()
	}
	for fr := range frames.Frames() {
		updates, ok := buildFrameBitmapUpdatesForDesktop(fr, cache, true, width, height)
		if !ok || len(updates) == 0 {
			continue
		}
		if err := writeBitmapUpdates(conn, updates); err != nil {
			tracef("frame_stream_stop", "err=%v", err)
			return
		}
	}
}

func writeBitmapUpdates(conn net.Conn, updates [][]byte) error {
	for _, update := range updates {
		if err := writeShareDataPDU(conn, pduType2Update, update); err != nil {
			return err
		}
	}
	return nil
}

func minPositive(v, max int) int {
	if v <= 0 || v > max {
		return max
	}
	return v
}

func parseShareDataPDU(share *shareControlPDU) (*shareDataPDU, error) {
	if share.PDUType != pduTypeData {
		return nil, fmt.Errorf("not Share Data PDU: 0x%04x", share.PDUType)
	}
	if len(share.Payload) < 12 {
		return nil, fmt.Errorf("short Share Data payload")
	}
	pdu := &shareDataPDU{
		ShareID:            binary.LittleEndian.Uint32(share.Payload[0:4]),
		StreamID:           share.Payload[5],
		UncompressedLength: binary.LittleEndian.Uint16(share.Payload[6:8]),
		PDUType2:           share.Payload[8],
		Payload:            share.Payload[12:],
	}
	return pdu, nil
}

func writeShareDataPDU(conn net.Conn, pduType2 uint8, payload []byte) error {
	tracef("share_data_write", "type2=0x%02x payload_len=%d", pduType2, len(payload))
	data := buildShareDataPDU(pduType2, payload)
	body := buildMCSSendDataIndication(serverChannelID, globalChannelID, data)
	return writeMCSDomainPDU(conn, mcsSendDataIndicationApp, body)
}

func buildShareDataPDU(pduType2 uint8, payload []byte) []byte {
	totalLength := 18 + len(payload)
	out := appendShareControlHeaderBytes(nil, totalLength, pduTypeData, serverChannelID)
	out = appendLE32Bytes(out, defaultShareID)
	out = append(out, 0x00) // pad1
	out = append(out, 0x01) // STREAM_LOW
	out = appendLE16Bytes(out, uint16(4+len(payload)))
	out = append(out, pduType2)
	out = append(out, 0x00)       // compressedType
	out = appendLE16Bytes(out, 0) // compressedLength
	out = append(out, payload...)
	return out
}

func buildSynchronizePayload() []byte {
	out := appendLE16Bytes(nil, 1)
	out = appendLE16Bytes(out, serverChannelID)
	return out
}

func buildControlPayload(action uint16) []byte {
	out := appendLE16Bytes(nil, action)
	out = appendLE16Bytes(out, 0)
	out = appendLE32Bytes(out, 0)
	return out
}

func buildFontMapPayload() []byte {
	out := appendLE16Bytes(nil, 0) // numberEntries
	out = appendLE16Bytes(out, 0)  // totalNumEntries
	out = appendLE16Bytes(out, 3)  // FONTMAP_FIRST | FONTMAP_LAST
	out = appendLE16Bytes(out, 4)  // entrySize
	return out
}

func parseControlAction(payload []byte) (uint16, error) {
	if len(payload) < 2 {
		return 0, fmt.Errorf("short Control PDU")
	}
	return binary.LittleEndian.Uint16(payload[0:2]), nil
}

func appendShareControlHeaderBytes(dst []byte, totalLength int, pduType uint16, source uint16) []byte {
	dst = appendLE16Bytes(dst, uint16(totalLength))
	dst = appendLE16Bytes(dst, pduType)
	dst = appendLE16Bytes(dst, source)
	return dst
}

func appendLE16Bytes(dst []byte, v uint16) []byte {
	return append(dst, byte(v), byte(v>>8))
}

func appendLE32Bytes(dst []byte, v uint32) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
