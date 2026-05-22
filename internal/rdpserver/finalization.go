package rdpserver

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

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

func handleShareDataPDU(conn net.Conn, share *shareControlPDU, frames frame.Source, h264 H264Source, sink input.Sink, width, height int, caps confirmActiveCapabilities, metrics serverMetrics, dvc *drdynvcManager) error {
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
		if dvc != nil && dvc.enabled() && rdpgfxEnabledFromEnv() {
			if err := dvc.startServerInitiatedChannels(conn); err != nil {
				return err
			}
			if err := waitForRDPGFXReady(conn, dvc, 750*time.Millisecond); err != nil {
				tracef("rdpgfx_wait", "ready=false err=%v", err)
			}
		}
		if dvc != nil && dvc.rdpgfxReady() {
			return writeInitialRDPGFXUpdate(conn, frames, h264, width, height, metrics, dvc)
		}
		return writeInitialBitmapUpdate(conn, frames, width, height, caps, metrics)
	case pduType2Input:
		return dispatchSlowPathInput(data.Payload, sink)
	}
	return nil
}

func writeInitialBitmapUpdate(conn net.Conn, frames frame.Source, width, height int, caps confirmActiveCapabilities, metrics serverMetrics) error {
	if frames != nil {
		select {
		case fr := <-frames.Frames():
			normalized := normalizeFrameForDesktop(fr, width, height)
			if codecID, ok := negotiatedNSCodecID(caps); ok {
				if command, built := buildNSCodecSurfaceBitsCommand(normalized, codecID); built {
					tracef("nscodec_selected", "codec_id=%d command_bytes=%d emission=opt-in", codecID, len(command))
					if err := writeShareDataPDU(conn, pduType2Update, command); err != nil {
						return err
					}
					metrics.recordNSCodecFrame([][]byte{command})
					tracef("nscodec_write", "codec_id=%d bytes=%d", codecID, len(command))
					return nil
				}
			}
			if codecID, ok := negotiatedJPEGCodecID(caps); ok {
				if command, built := buildJPEGSurfaceBitsCommand(normalized, codecID, 80); built {
					tracef("jpeg_codec_selected", "codec_id=%d command_bytes=%d emission=opt-in", codecID, len(command))
					if err := writeShareDataPDU(conn, pduType2Update, command); err != nil {
						return err
					}
					metrics.recordJPEGCodecFrame([][]byte{command})
					tracef("jpeg_codec_write", "codec_id=%d bytes=%d", codecID, len(command))
					return nil
				}
			}
			cache := newBitmapTileCache()
			if updates, ok := buildFrameBitmapUpdatesForDesktop(fr, cache, false, width, height); ok {
				if err := writeBitmapUpdates(conn, updates); err != nil {
					return err
				}
				metrics.recordBitmapFrame(updates)
				go streamFrameUpdates(conn, frames, cache, width, height, metrics)
				return nil
			}
		default:
		}
	}
	update := buildSolidBitmapUpdate(minPositive(width, 64), minPositive(height, 64), 0xff336699)
	if err := writeShareDataPDU(conn, pduType2Update, update); err != nil {
		return err
	}
	metrics.recordBitmapFrame([][]byte{update})
	return nil
}

func rdpgfxEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_DISABLE_RDPGFX"))) {
	case "1", "true", "yes", "on":
		return false
	default:
		return true
	}
}

func waitForRDPGFXReady(conn net.Conn, dvc *drdynvcManager, timeout time.Duration) error {
	if dvc == nil || !dvc.enabled() || dvc.rdpgfxReady() {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) && !dvc.rdpgfxReady() {
		_ = conn.SetReadDeadline(deadline)
		pdu, err := readMCSDomainPDUOrFastPath(conn, nil)
		if err != nil {
			if errors.Is(err, errFastPathPDU) {
				continue
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return err
			}
			return err
		}
		if pdu.Application == mcsSendDataRequestApp && pdu.ChannelID == dvc.staticChannelID {
			if err := dvc.handleStaticPDU(conn, pdu.Data); err != nil {
				return err
			}
		}
	}
	_ = conn.SetReadDeadline(time.Time{})
	if !dvc.rdpgfxReady() {
		return fmt.Errorf("RDPGFX channel not ready")
	}
	return nil
}

func writeInitialRDPGFXUpdate(conn net.Conn, frames frame.Source, h264 H264Source, width, height int, metrics serverMetrics, dvc *drdynvcManager) error {
	create, ok := buildRDPGFXCreateSurfacePDU(0, width, height)
	if !ok {
		return writeInitialBitmapUpdate(conn, frames, width, height, confirmActiveCapabilities{}, metrics)
	}
	mapped, ok := buildRDPGFXMapSurfaceToOutputPDU(0, 0, 0)
	if !ok {
		return writeInitialBitmapUpdate(conn, frames, width, height, confirmActiveCapabilities{}, metrics)
	}
	if err := dvc.writeRDPGFXPayload(conn, create); err != nil {
		return err
	}
	if err := dvc.writeRDPGFXPayload(conn, mapped); err != nil {
		return err
	}
	nextFrameID := uint32(1)
	var h264State h264StreamState
	h264Ready, h264Version, h264Flags, h264Reason := dvc.rdpgfxH264Status()
	metrics.recordH264Status(h264Reason)
	tracef("rdpgfx_h264_status", "ready=%t version=0x%08x flags=0x%08x reason=%s source=%t", h264Ready, h264Version, h264Flags, h264Reason, h264 != nil)
	if h264 != nil && h264Ready {
		select {
		case unit := <-h264.H264Frames():
			unit = latestAvailableH264Unit(h264.H264Frames(), unit, &h264State)
			wireUnit, ready := h264State.prepareForWire(unit)
			if ready {
				pdus, ok := buildRDPGFXH264FramePDUs(0, nextFrameID, wireUnit, width, height)
				if ok {
					for _, pdu := range pdus {
						if err := dvc.writeRDPGFXPayload(conn, pdu); err != nil {
							return err
						}
					}
					metrics.recordH264Frame([][]byte{wireUnit.Data})
					tracef("rdpgfx_h264_write", "frame_id=%d pts=%d key=%t config=%t bytes=%d", nextFrameID, wireUnit.PresentationTimeUS, wireUnit.KeyFrame, wireUnit.CodecConfig, len(wireUnit.Data))
					go streamRDPGFXH264Updates(conn, h264, dvc, width, height, metrics, nextFrameID+1, h264State)
					return nil
				}
			}
		default:
		}
		go streamRDPGFXH264Updates(conn, h264, dvc, width, height, metrics, nextFrameID, h264State)
	}
	if frames != nil {
		select {
		case fr := <-frames.Frames():
			pdus, ok := buildRDPGFXPlanarFramePDUs(0, 1, fr, width, height)
			if ok {
				for _, pdu := range pdus {
					if err := dvc.writeRDPGFXPayload(conn, pdu); err != nil {
						return err
					}
				}
				metrics.recordRDPGFXFrame(pdus)
				return nil
			}
		default:
		}
	}
	return nil
}

func streamRDPGFXH264Updates(conn net.Conn, h264 H264Source, dvc *drdynvcManager, width, height int, metrics serverMetrics, nextFrameID uint32, state h264StreamState) {
	if h264 == nil || dvc == nil || !dvc.rdpgfxH264Ready() {
		return
	}
	frameCh := h264.H264Frames()
	for unit := range frameCh {
		unit = latestAvailableH264Unit(frameCh, unit, &state)
		wireUnit, ready := state.prepareForWire(unit)
		if !ready {
			continue
		}
		pdus, ok := buildRDPGFXH264FramePDUs(0, nextFrameID, wireUnit, width, height)
		if !ok || len(pdus) == 0 {
			continue
		}
		for _, pdu := range pdus {
			if err := dvc.writeRDPGFXPayload(conn, pdu); err != nil {
				tracef("rdpgfx_h264_stream_stop", "err=%v", err)
				return
			}
		}
		metrics.recordH264Frame([][]byte{wireUnit.Data})
		tracef("rdpgfx_h264_write", "frame_id=%d pts=%d key=%t config=%t bytes=%d", nextFrameID, wireUnit.PresentationTimeUS, wireUnit.KeyFrame, wireUnit.CodecConfig, len(wireUnit.Data))
		nextFrameID++
	}
}

func streamFrameUpdates(conn net.Conn, frames frame.Source, cache *bitmapTileCache, width, height int, metrics serverMetrics) {
	if cache == nil {
		cache = newBitmapTileCache()
	}
	frameCh := frames.Frames()
	for fr := range frameCh {
		fr = latestAvailableFrame(frameCh, fr)
		updates, ok := buildFrameBitmapUpdatesForDesktop(fr, cache, true, width, height)
		if !ok || len(updates) == 0 {
			continue
		}
		if err := writeBitmapUpdates(conn, updates); err != nil {
			tracef("frame_stream_stop", "err=%v", err)
			return
		}
		metrics.recordBitmapFrame(updates)
	}
}

func latestAvailableH264Unit(frameCh <-chan H264Frame, current H264Frame, state *h264StreamState) H264Frame {
	latest := current
	if state != nil && current.CodecConfig && !current.KeyFrame {
		_, _ = state.prepareForWire(current)
		latest = H264Frame{}
	}
	for {
		select {
		case unit, ok := <-frameCh:
			if !ok {
				return latest
			}
			if state != nil && unit.CodecConfig && !unit.KeyFrame {
				_, _ = state.prepareForWire(unit)
				continue
			}
			latest = unit
		default:
			return latest
		}
	}
}

func latestAvailableFrame(frameCh <-chan frame.Frame, current frame.Frame) frame.Frame {
	latest := current
	for {
		select {
		case fr, ok := <-frameCh:
			if !ok {
				return latest
			}
			latest = fr
		default:
			return latest
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
