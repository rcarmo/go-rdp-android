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
	bitmapLen := 4 + 8 + 2 + len(unit.Data)
	wireLen := rdpgfxHeaderLen + rdpgfxWireToSurface1PayloadHeaderLen + bitmapLen
	backing := make([]byte, 16+wireLen+12)
	start := backing[:16]
	writeRDPGFXPDUHeader(start, rdpgfxCmdStartFrame, 0)
	binary.LittleEndian.PutUint32(start[8:12], uint32(0))
	binary.LittleEndian.PutUint32(start[12:16], frameID)
	wire := backing[16 : 16+wireLen]
	writeRDPGFXWireToSurface1Header(wire, surfaceID, rdpgfxCodecAVC420, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(width), uint16(height), bitmapLen) // #nosec G115 -- dimensions bounded above.
	bitmap := wire[rdpgfxHeaderLen+rdpgfxWireToSurface1PayloadHeaderLen:]
	binary.LittleEndian.PutUint32(bitmap[0:4], 1) // numRegionRects
	binary.LittleEndian.PutUint16(bitmap[4:6], 0) // left
	binary.LittleEndian.PutUint16(bitmap[6:8], 0) // top
	binary.LittleEndian.PutUint16(bitmap[8:10], uint16(width))
	binary.LittleEndian.PutUint16(bitmap[10:12], uint16(height))
	bitmap[12] = 0 // qpVal
	bitmap[13] = 0 // qualityVal
	copy(bitmap[14:], unit.Data)
	end := backing[16+wireLen:]
	writeRDPGFXPDUHeader(end, rdpgfxCmdEndFrame, 0)
	binary.LittleEndian.PutUint32(end[8:12], frameID)
	return [][]byte{start, wire, end}, true
}

func buildRDPGFXAVC420BitmapStream(accessUnit []byte, width, height uint16) []byte {
	payload := make([]byte, 0, 4+8+2+len(accessUnit))
	payload = appendLE32Bytes(payload, 1) // numRegionRects
	payload = appendLE16Bytes(payload, 0) // left
	payload = appendLE16Bytes(payload, 0) // top
	payload = appendLE16Bytes(payload, width)
	payload = appendLE16Bytes(payload, height)
	payload = append(payload, 0) // qpVal
	payload = append(payload, 0) // qualityVal
	payload = append(payload, accessUnit...)
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
	bitmapStart := rdpgfxHeaderLen + rdpgfxWireToSurface1PayloadHeaderLen
	planeSize := src.Width * src.Height
	payloadCap := 1 + planeSize*3 + planeSize/4 + 64
	backing := make([]byte, 16+bitmapStart, 16+bitmapStart+payloadCap+planeSize+12)
	start := backing[:16]
	writeRDPGFXPDUHeader(start, rdpgfxCmdStartFrame, 0)
	binary.LittleEndian.PutUint32(start[8:12], uint32(0))
	binary.LittleEndian.PutUint32(start[12:16], frameID)
	wire := backing[16 : 16+bitmapStart : 16+bitmapStart+payloadCap]
	plane := backing[16+bitmapStart+payloadCap : 16+bitmapStart+payloadCap+planeSize]
	writeRDPGFXWireToSurface1Header(wire, surfaceID, rdpgfxCodecPlanar, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(src.Width), uint16(src.Height), 0) // #nosec G115 -- dimensions validated by normalizedFrameStride and desktop clamp.
	wire, ok := appendPlanarRLEPayloadWithPlane(wire, src, stride, plane)
	if !ok {
		return nil, nil, nil, false
	}
	binary.LittleEndian.PutUint32(wire[21:25], uint32(len(wire)-bitmapStart)) // #nosec G115 -- payload length is bounded by allocation.
	binary.LittleEndian.PutUint32(wire[4:8], uint32(len(wire)))               // #nosec G115 -- PDU length is bounded by allocation.
	endStart := 16 + len(wire)
	end := backing[endStart : endStart+12]
	writeRDPGFXPDUHeader(end, rdpgfxCmdEndFrame, 0)
	binary.LittleEndian.PutUint32(end[8:12], frameID)
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
	bitmapStart := rdpgfxHeaderLen + rdpgfxWireToSurface1PayloadHeaderLen
	planeSize := src.Width * src.Height
	payloadCap := 1 + planeSize*3
	wire := make([]byte, bitmapStart, bitmapStart+payloadCap+planeSize)
	plane := wire[bitmapStart+payloadCap : bitmapStart+payloadCap+planeSize]
	writeRDPGFXWireToSurface1Header(wire, surfaceID, rdpgfxCodecPlanar, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(src.Width), uint16(src.Height), 0) // #nosec G115 -- dimensions validated by normalizedFrameStride and desktop clamp.
	wire, ok := appendPlanarRLEPayloadWithPlane(wire, src, stride, plane)
	if !ok {
		return nil, false
	}
	binary.LittleEndian.PutUint32(wire[21:25], uint32(len(wire)-bitmapStart)) // #nosec G115 -- payload length is bounded by allocation.
	binary.LittleEndian.PutUint32(wire[4:8], uint32(len(wire)))               // #nosec G115 -- PDU length is bounded by allocation.
	return wire, true
}

func buildPlanarRLEPayload(src frame.Frame, stride int) ([]byte, bool) {
	maxInt := int(^uint(0) >> 1)
	if src.Width <= 0 || src.Height <= 0 || src.Width > maxInt/src.Height {
		return nil, false
	}
	out := make([]byte, 0, 1+src.Width*src.Height*3)
	return appendPlanarRLEPayload(out, src, stride)
}

func appendPlanarRLEPayload(out []byte, src frame.Frame, stride int) ([]byte, bool) {
	maxInt := int(^uint(0) >> 1)
	if src.Width <= 0 || src.Height <= 0 || src.Width > maxInt/src.Height {
		return nil, false
	}
	plane := make([]byte, src.Width*src.Height)
	return appendPlanarRLEPayloadWithPlane(out, src, stride, plane)
}

func appendPlanarRLEPayloadWithPlane(out []byte, src frame.Frame, stride int, plane []byte) ([]byte, bool) {
	maxInt := int(^uint(0) >> 1)
	if src.Width <= 0 || src.Height <= 0 || src.Width > maxInt/src.Height || len(plane) < src.Width*src.Height {
		return nil, false
	}
	plane = plane[:src.Width*src.Height]
	out = append(out, 0x30) // PLANAR_FORMAT_HEADER_NA | PLANAR_FORMAT_HEADER_RLE.
	for i := 0; i < 3; i++ {
		component := "rgb"[i]
		if !fillPlanarColorPlane(plane, src, stride, component) {
			return nil, false
		}
		out = appendPlanarDeltaRLEPlane(out, plane, src.Width, src.Height)
	}
	return out, true
}

func fillPlanarColorPlane(plane []byte, src frame.Frame, stride int, component byte) bool {
	for y := 0; y < src.Height; y++ {
		for x := 0; x < src.Width; x++ {
			si := y*stride + x*4
			di := y*src.Width + x
			switch src.Format {
			case frame.PixelFormatBGRA8888:
				switch component {
				case 'r':
					plane[di] = src.Data[si+2]
				case 'g':
					plane[di] = src.Data[si+1]
				case 'b':
					plane[di] = src.Data[si+0]
				default:
					return false
				}
			case frame.PixelFormatRGBA8888:
				switch component {
				case 'r':
					plane[di] = src.Data[si+0]
				case 'g':
					plane[di] = src.Data[si+1]
				case 'b':
					plane[di] = src.Data[si+2]
				default:
					return false
				}
			default:
				return false
			}
		}
	}
	return true
}

func appendPlanarDeltaRLEPlane(out, plane []byte, width, height int) []byte {
	for y := height - 1; y > 0; y-- {
		row := plane[y*width : y*width+width]
		prev := plane[(y-1)*width : (y-1)*width+width]
		for x := 0; x < width; x++ {
			row[x] = planarDeltaByte(int(row[x]) - int(prev[x]))
		}
	}
	for y := 0; y < height; y++ {
		out = appendPlanarRLELine(out, plane[y*width:y*width+width])
	}
	return out
}

func planarDeltaByte(delta int) byte {
	if delta > 127 {
		delta -= 256
	} else if delta < -128 {
		delta += 256
	}
	if delta >= 0 {
		return byte(delta << 1)
	}
	return byte(((-delta) << 1) - 1)
}

func appendPlanarRLELine(out []byte, row []byte) []byte {
	for x := 0; x < len(row); {
		if row[x] == 0 {
			run := 1
			for x+run < len(row) && row[x+run] == 0 && run < 47 {
				run++
			}
			if run >= 16 {
				if run < 32 {
					out = append(out, byte(((run-16)&0x0f)<<4|0x01))
				} else {
					out = append(out, byte(((run-32)&0x0f)<<4|0x02))
				}
				x += run
				continue
			}
		}
		rawStart := x
		rawLen := 0
		for x < len(row) && rawLen < 15 {
			if row[x] == 0 {
				run := 1
				for x+run < len(row) && row[x+run] == 0 && run < 16 {
					run++
				}
				if run >= 16 {
					break
				}
			}
			x++
			rawLen++
		}
		out = append(out, byte(rawLen<<4))
		out = append(out, row[rawStart:rawStart+rawLen]...)
	}
	return out
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
