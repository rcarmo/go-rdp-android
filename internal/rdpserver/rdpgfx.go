package rdpserver

import (
	"encoding/binary"
	"fmt"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

const (
	rdpgfxDynamicChannelName = "Microsoft::Windows::RDS::Graphics"

	rdpgfxCmdWireToSurface1     uint16 = rdpcodec.RDPGFXCmdWireToSurface1
	rdpgfxCmdCreateSurface      uint16 = rdpcodec.RDPGFXCmdCreateSurface
	rdpgfxCmdStartFrame         uint16 = rdpcodec.RDPGFXCmdStartFrame
	rdpgfxCmdEndFrame           uint16 = rdpcodec.RDPGFXCmdEndFrame
	rdpgfxCmdResetGraphics      uint16 = rdpcodec.RDPGFXCmdResetGraphics
	rdpgfxCmdMapSurfaceToOutput uint16 = rdpcodec.RDPGFXCmdMapSurfaceToOutput
	rdpgfxCmdCapsAdvertise      uint16 = rdpcodec.RDPGFXCmdCapsAdvertise
	rdpgfxCmdCapsConfirm        uint16 = rdpcodec.RDPGFXCmdCapsConfirm

	rdpgfxCodecUncompressed    uint16 = rdpcodec.RDPGFXCodecUncompressed
	rdpgfxCodecClearCodec      uint16 = rdpcodec.RDPGFXCodecClearCodec
	rdpgfxCodecCAProgressive   uint16 = rdpcodec.RDPGFXCodecCAProgressive
	rdpgfxCodecPlanar          uint16 = rdpcodec.RDPGFXCodecPlanar
	rdpgfxCodecAVC420          uint16 = rdpcodec.RDPGFXCodecAVC420
	rdpgfxCodecCAProgressiveV2 uint16 = rdpcodec.RDPGFXCodecCAProgressiveV2
	rdpgfxCodecAVC444          uint16 = rdpcodec.RDPGFXCodecAVC444
	rdpgfxCodecAVC444v2        uint16 = rdpcodec.RDPGFXCodecAVC444v2

	rdpgfxPixelFormatXRGB8888 byte = rdpcodec.RDPGFXPixelFormatXRGB8888

	rdpgfxCapsFlagAVC420Enabled uint32 = rdpcodec.RDPGFXCapsFlagAVC420Enabled
	rdpgfxCapsFlagAVCDisabled   uint32 = rdpcodec.RDPGFXCapsFlagAVCDisabled

	rdpgfxCapsVersion8   uint32 = rdpcodec.RDPGFXCapsVersion8
	rdpgfxCapsVersion81  uint32 = rdpcodec.RDPGFXCapsVersion81
	rdpgfxCapsVersion10  uint32 = rdpcodec.RDPGFXCapsVersion10
	rdpgfxCapsVersion102 uint32 = rdpcodec.RDPGFXCapsVersion102
	rdpgfxCapsVersion103 uint32 = rdpcodec.RDPGFXCapsVersion103
	rdpgfxCapsVersion104 uint32 = rdpcodec.RDPGFXCapsVersion104
	rdpgfxCapsVersion105 uint32 = rdpcodec.RDPGFXCapsVersion105
	rdpgfxCapsVersion106 uint32 = rdpcodec.RDPGFXCapsVersion106

	rdpgfxMaxPDUSize = 1024 * 1024
)

type rdpgfxPDU struct {
	CmdID  uint16
	Flags  uint16
	Length uint32
	Caps   []rdpgfxCapabilitySet
}

type rdpgfxCapabilitySet struct {
	Version        uint32
	CapsDataLength uint32
	Flags          uint32
}

func parseRDPGFXPDU(data []byte) (*rdpgfxPDU, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("short RDPGFX PDU")
	}
	if len(data) > rdpgfxMaxPDUSize {
		return nil, fmt.Errorf("RDPGFX PDU length %d exceeds maximum %d", len(data), rdpgfxMaxPDUSize)
	}
	pdu := &rdpgfxPDU{
		CmdID:  binary.LittleEndian.Uint16(data[0:2]),
		Flags:  binary.LittleEndian.Uint16(data[2:4]),
		Length: binary.LittleEndian.Uint32(data[4:8]),
	}
	if pdu.Length != uint32(len(data)) {
		return nil, fmt.Errorf("RDPGFX PDU length mismatch: header=%d payload=%d", pdu.Length, len(data))
	}
	switch pdu.CmdID {
	case rdpgfxCmdCapsAdvertise:
		caps, err := parseRDPGFXCapsAdvertise(data[8:])
		if err != nil {
			return nil, err
		}
		pdu.Caps = caps
	}
	return pdu, nil
}

func parseRDPGFXCapsAdvertise(data []byte) ([]rdpgfxCapabilitySet, error) {
	upstreamCaps, err := rdpcodec.ParseRDPGFXCapsAdvertise(data)
	if err != nil {
		return nil, err
	}
	caps := make([]rdpgfxCapabilitySet, 0, len(upstreamCaps))
	for _, cap := range upstreamCaps {
		caps = append(caps, rdpgfxCapabilitySet{Version: cap.Version, CapsDataLength: cap.CapsDataLength, Flags: cap.Flags})
	}
	return caps, nil
}

func negotiateRDPGFXCapability(caps []rdpgfxCapabilitySet) (rdpgfxCapabilitySet, bool) {
	var best rdpgfxCapabilitySet
	for _, cap := range caps {
		if !supportedRDPGFXVersion(cap.Version) {
			continue
		}
		if best.Version == 0 || cap.Version > best.Version {
			best = cap
		}
	}
	return best, best.Version != 0
}

func rdpgfxCapabilitySupportsH264(cap rdpgfxCapabilitySet) bool {
	if !h264EnabledFromEnv() {
		return false
	}
	if h264ForcedFromEnv() {
		return cap.Version >= rdpgfxCapsVersion81
	}
	if cap.Version == rdpgfxCapsVersion81 {
		return cap.Flags&rdpgfxCapsFlagAVC420Enabled != 0
	}
	if cap.Version >= rdpgfxCapsVersion10 {
		return cap.Flags&rdpgfxCapsFlagAVCDisabled == 0
	}
	return false
}

func supportedRDPGFXVersion(version uint32) bool {
	switch version {
	case rdpgfxCapsVersion8, rdpgfxCapsVersion81, rdpgfxCapsVersion10, rdpgfxCapsVersion102, rdpgfxCapsVersion103, rdpgfxCapsVersion104, rdpgfxCapsVersion105, rdpgfxCapsVersion106:
		return true
	default:
		return false
	}
}

func buildRDPGFXCapsConfirmPDU(cap rdpgfxCapabilitySet) []byte {
	payload := make([]byte, 12)
	binary.LittleEndian.PutUint32(payload[0:4], cap.Version)
	binary.LittleEndian.PutUint32(payload[4:8], 4)
	binary.LittleEndian.PutUint32(payload[8:12], cap.Flags)
	return buildRDPGFXPDU(rdpgfxCmdCapsConfirm, 0, payload)
}

func buildRDPGFXCreateSurfacePDU(surfaceID uint16, width, height int) ([]byte, bool) {
	if width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return nil, false
	}
	pdu, err := rdpcodec.BuildRDPGFXCreateSurface(surfaceID, uint16(width), uint16(height), rdpgfxPixelFormatXRGB8888) // #nosec G115 -- dimensions bounded above.
	if err != nil {
		return nil, false
	}
	return pdu, true
}

func buildRDPGFXMapSurfaceToOutputPDU(surfaceID uint16, originX, originY int) ([]byte, bool) {
	if originX < 0 || originY < 0 || originX > 32767 || originY > 32767 {
		return nil, false
	}
	pdu, err := rdpcodec.BuildRDPGFXMapSurfaceToOutput(surfaceID, uint32(originX), uint32(originY)) // #nosec G115 -- bounded above.
	if err != nil {
		return nil, false
	}
	return pdu, true
}

func buildRDPGFXStartFramePDU(frameID uint32) []byte {
	out, err := rdpcodec.BuildRDPGFXStartFrame(frameID)
	if err != nil {
		return nil
	}
	return out
}

func buildRDPGFXEndFramePDU(frameID uint32) []byte {
	out, err := rdpcodec.BuildRDPGFXEndFrame(frameID)
	if err != nil {
		return nil
	}
	return out
}

func buildRDPGFXWireToSurface1PDU(surfaceID uint16, codecID uint16, pixelFormat byte, destLeft, destTop, destRight, destBottom uint16, bitmapData []byte) []byte {
	out, err := rdpcodec.BuildRDPGFXWireToSurface1(surfaceID, codecID, pixelFormat, rdpcodec.Rect{Left: destLeft, Top: destTop, Right: destRight, Bottom: destBottom}, bitmapData)
	if err != nil {
		return nil
	}
	return out
}

const (
	rdpgfxHeaderLen                      = 8
	rdpgfxWireToSurface1PayloadHeaderLen = 17
)

func writeRDPGFXWireToSurface1Header(out []byte, surfaceID uint16, codecID uint16, pixelFormat byte, destLeft, destTop, destRight, destBottom uint16, bitmapLen int) {
	writeRDPGFXPDUHeader(out, rdpgfxCmdWireToSurface1, 0)
	binary.LittleEndian.PutUint16(out[8:10], surfaceID)
	binary.LittleEndian.PutUint16(out[10:12], codecID)
	out[12] = pixelFormat
	binary.LittleEndian.PutUint16(out[13:15], destLeft)
	binary.LittleEndian.PutUint16(out[15:17], destTop)
	binary.LittleEndian.PutUint16(out[17:19], destRight)
	binary.LittleEndian.PutUint16(out[19:21], destBottom)
	binary.LittleEndian.PutUint32(out[21:25], uint32(bitmapLen)) // #nosec G115 -- payload length is bounded by allocation.
}

func buildRDPGFXH264FramePDUs(surfaceID uint16, frameID uint32, unit h264AccessUnit, width, height int) ([][]byte, bool) {
	if width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return nil, false
	}
	if err := validateH264AccessUnit(unit); err != nil {
		return nil, false
	}
	start, err := rdpcodec.BuildRDPGFXStartFrame(frameID)
	if err != nil {
		return nil, false
	}
	wire, err := rdpcodec.BuildAVC420WireToSurface(surfaceID, rdpgfxPixelFormatXRGB8888, rdpcodec.Rect{Right: uint16(width - 1), Bottom: uint16(height - 1)}, unit.Data, uint16(width), uint16(height)) // #nosec G115 -- dimensions bounded above.
	if err != nil {
		return nil, false
	}
	end, err := rdpcodec.BuildRDPGFXEndFrame(frameID)
	if err != nil {
		return nil, false
	}
	return [][]byte{start, wire, end}, true
}

func buildRDPGFXAVC420BitmapStream(accessUnit []byte, width, height uint16) []byte {
	payload, err := rdpcodec.BuildAVC420BitmapStream(accessUnit, width, height)
	if err != nil {
		return nil
	}
	return payload
}

func buildRDPGFXPlanarFramePDUs(surfaceID uint16, frameID uint32, src frame.Frame, width, height int) ([][]byte, bool) {
	normalized := normalizeFrameForDesktop(src, width, height)
	stride, ok := normalizedFrameStride(normalized)
	if !ok || normalized.Format != frame.PixelFormatRGBA8888 && normalized.Format != frame.PixelFormatBGRA8888 {
		return nil, false
	}
	maxInt := int(^uint(0) >> 1)
	if normalized.Width > maxInt/4 || normalized.Height > maxInt/(normalized.Width*4) {
		return nil, false
	}
	start, wire, end, ok := buildRDPGFXPlanarFramePDUBacking(surfaceID, frameID, normalized, stride)
	if !ok {
		return nil, false
	}
	return [][]byte{start, wire, end}, true
}

func buildRDPGFXPlanarFramePDUBacking(surfaceID uint16, frameID uint32, src frame.Frame, stride int) ([]byte, []byte, []byte, bool) {
	payload, ok := buildPlanarRLEPayload(src, stride)
	if !ok {
		return nil, nil, nil, false
	}
	start, err := rdpcodec.BuildRDPGFXStartFrame(frameID)
	if err != nil {
		return nil, nil, nil, false
	}
	wire, err := rdpcodec.BuildRDPGFXWireToSurface1(surfaceID, rdpgfxCodecPlanar, rdpgfxPixelFormatXRGB8888, rdpcodec.Rect{Right: uint16(src.Width - 1), Bottom: uint16(src.Height - 1)}, payload) // #nosec G115 -- dimensions validated by normalizedFrameStride and desktop clamp.
	if err != nil {
		return nil, nil, nil, false
	}
	end, err := rdpcodec.BuildRDPGFXEndFrame(frameID)
	if err != nil {
		return nil, nil, nil, false
	}
	return start, wire, end, true
}

func buildRDPGFXFrameBoundaryPDUs(frameID uint32) ([]byte, []byte) {
	out := make([]byte, 28)
	start := out[:16]
	writeRDPGFXPDUHeader(start, rdpgfxCmdStartFrame, 0)
	binary.LittleEndian.PutUint32(start[8:12], uint32(0))
	binary.LittleEndian.PutUint32(start[12:16], frameID)
	end := out[16:28]
	writeRDPGFXPDUHeader(end, rdpgfxCmdEndFrame, 0)
	binary.LittleEndian.PutUint32(end[8:12], frameID)
	return start, end
}

func buildRDPGFXPlanarWireToSurfacePDU(surfaceID uint16, src frame.Frame, stride int) ([]byte, bool) {
	payload, ok := buildPlanarRLEPayload(src, stride)
	if !ok {
		return nil, false
	}
	wire, err := rdpcodec.BuildRDPGFXWireToSurface1(surfaceID, rdpgfxCodecPlanar, rdpgfxPixelFormatXRGB8888, rdpcodec.Rect{Right: uint16(src.Width - 1), Bottom: uint16(src.Height - 1)}, payload) // #nosec G115 -- dimensions validated by normalizedFrameStride and desktop clamp.
	if err != nil {
		return nil, false
	}
	return wire, true
}

func buildPlanarRLEPayload(src frame.Frame, stride int) ([]byte, bool) {
	format, ok := planarPixelFormat(src.Format)
	if !ok {
		return nil, false
	}
	payload, err := rdpcodec.EncodePlanarNoAlpha(rdpcodec.PlanarInput{
		Pixels: src.Data,
		Width:  src.Width,
		Height: src.Height,
		Stride: stride,
		Format: format,
	})
	if err != nil {
		return nil, false
	}
	return payload, true
}

func appendPlanarRLEPayload(out []byte, src frame.Frame, stride int) ([]byte, bool) {
	payload, ok := buildPlanarRLEPayload(src, stride)
	if !ok {
		return nil, false
	}
	return append(out, payload...), true
}

func appendPlanarRLEPayloadWithPlane(out []byte, src frame.Frame, stride int, _ []byte) ([]byte, bool) {
	return appendPlanarRLEPayload(out, src, stride)
}

func planarPixelFormat(format frame.PixelFormat) (rdpcodec.PixelFormat, bool) {
	switch format {
	case frame.PixelFormatRGBA8888:
		return rdpcodec.PixelFormatRGBA, true
	case frame.PixelFormatBGRA8888:
		return rdpcodec.PixelFormatBGRA, true
	default:
		return 0, false
	}
}

func planarDeltaByte(delta int) byte {
	return rdpcodec.PlanarDeltaByte(delta)
}

func buildRDPGFXPDU(cmdID, flags uint16, payload []byte) []byte {
	out, err := rdpcodec.BuildRDPGFXPDU(cmdID, flags, payload)
	if err != nil {
		return nil
	}
	return out
}

func writeRDPGFXPDUHeader(out []byte, cmdID, flags uint16) {
	pdu, err := rdpcodec.BuildRDPGFXPDU(cmdID, flags, out[8:])
	if err != nil {
		return
	}
	copy(out[:8], pdu[:8])
}

func traceRDPGFXPDU(pdu *rdpgfxPDU) {
	if pdu == nil {
		return
	}
	switch pdu.CmdID {
	case rdpgfxCmdCapsAdvertise:
		tracef("rdpgfx_caps_advertise", "caps=%d", len(pdu.Caps))
		for i, cap := range pdu.Caps {
			tracef("rdpgfx_cap", "index=%d version=0x%08x caps_data_len=%d flags=0x%08x supported=%t", i, cap.Version, cap.CapsDataLength, cap.Flags, supportedRDPGFXVersion(cap.Version))
		}
	default:
		tracef("rdpgfx_pdu", "cmd=0x%04x flags=0x%04x length=%d", pdu.CmdID, pdu.Flags, pdu.Length)
	}
}
