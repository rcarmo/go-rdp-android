package rdpserver

import (
	"encoding/binary"
	"fmt"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

const (
	rdpgfxDynamicChannelName = "Microsoft::Windows::RDS::Graphics"

	rdpgfxCmdWireToSurface1     uint16 = 0x0001
	rdpgfxCmdCreateSurface      uint16 = 0x0009
	rdpgfxCmdStartFrame         uint16 = 0x000b
	rdpgfxCmdEndFrame           uint16 = 0x000c
	rdpgfxCmdResetGraphics      uint16 = 0x000e
	rdpgfxCmdMapSurfaceToOutput uint16 = 0x000f
	rdpgfxCmdCapsAdvertise      uint16 = 0x0012
	rdpgfxCmdCapsConfirm        uint16 = 0x0013

	rdpgfxCodecUncompressed    uint16 = rdpcodec.RDPGFXCodecUncompressed
	rdpgfxCodecClearCodec      uint16 = rdpcodec.RDPGFXCodecClearCodec
	rdpgfxCodecCAProgressive   uint16 = rdpcodec.RDPGFXCodecCAProgressive
	rdpgfxCodecPlanar          uint16 = rdpcodec.RDPGFXCodecPlanar
	rdpgfxCodecAVC420          uint16 = rdpcodec.RDPGFXCodecAVC420
	rdpgfxCodecCAProgressiveV2 uint16 = rdpcodec.RDPGFXCodecCAProgressiveV2
	rdpgfxCodecAVC444          uint16 = rdpcodec.RDPGFXCodecAVC444
	rdpgfxCodecAVC444v2        uint16 = rdpcodec.RDPGFXCodecAVC444v2

	rdpgfxPixelFormatXRGB8888 byte = 0x20

	rdpgfxCapsFlagAVC420Enabled uint32 = 0x00000010
	rdpgfxCapsFlagAVCDisabled   uint32 = 0x00000020

	rdpgfxCapsVersion8   uint32 = 0x00080004
	rdpgfxCapsVersion81  uint32 = 0x00080105
	rdpgfxCapsVersion10  uint32 = 0x000A0002
	rdpgfxCapsVersion102 uint32 = 0x000A0200
	rdpgfxCapsVersion103 uint32 = 0x000A0301
	rdpgfxCapsVersion104 uint32 = 0x000A0400
	rdpgfxCapsVersion105 uint32 = 0x000A0502
	rdpgfxCapsVersion106 uint32 = 0x000A0600

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
	if len(data) < 2 {
		return nil, fmt.Errorf("short RDPGFX caps advertise")
	}
	count := int(binary.LittleEndian.Uint16(data[0:2]))
	data = data[2:]
	if count == 0 {
		return nil, fmt.Errorf("RDPGFX caps advertise contains no capability sets")
	}
	if count > 64 {
		return nil, fmt.Errorf("RDPGFX caps count %d exceeds maximum 64", count)
	}
	caps := make([]rdpgfxCapabilitySet, 0, count)
	offset := 0
	for i := 0; i < count; i++ {
		if len(data)-offset < 8 {
			return nil, fmt.Errorf("short RDPGFX capability set header %d", i)
		}
		version := binary.LittleEndian.Uint32(data[offset : offset+4])
		capsDataLength := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8
		if capsDataLength > 64 || int(capsDataLength) > len(data)-offset {
			return nil, fmt.Errorf("invalid RDPGFX capability set %d data length %d", i, capsDataLength)
		}
		var flags uint32
		if capsDataLength >= 4 {
			flags = binary.LittleEndian.Uint32(data[offset : offset+4])
		}
		offset += int(capsDataLength)
		caps = append(caps, rdpgfxCapabilitySet{Version: version, CapsDataLength: capsDataLength, Flags: flags})
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
	payload := make([]byte, 7)
	binary.LittleEndian.PutUint16(payload[0:2], surfaceID)
	binary.LittleEndian.PutUint16(payload[2:4], uint16(width))  // #nosec G115 -- bounded above.
	binary.LittleEndian.PutUint16(payload[4:6], uint16(height)) // #nosec G115 -- bounded above.
	payload[6] = rdpgfxPixelFormatXRGB8888
	return buildRDPGFXPDU(rdpgfxCmdCreateSurface, 0, payload), true
}

func buildRDPGFXMapSurfaceToOutputPDU(surfaceID uint16, originX, originY int) ([]byte, bool) {
	if originX < 0 || originY < 0 || originX > 32767 || originY > 32767 {
		return nil, false
	}
	payload := make([]byte, 12)
	binary.LittleEndian.PutUint16(payload[0:2], surfaceID)
	binary.LittleEndian.PutUint16(payload[2:4], 0)                // reserved
	binary.LittleEndian.PutUint32(payload[4:8], uint32(originX))  // #nosec G115 -- bounded above.
	binary.LittleEndian.PutUint32(payload[8:12], uint32(originY)) // #nosec G115 -- bounded above.
	return buildRDPGFXPDU(rdpgfxCmdMapSurfaceToOutput, 0, payload), true
}

func buildRDPGFXStartFramePDU(frameID uint32) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload[0:4], uint32(0))
	binary.LittleEndian.PutUint32(payload[4:8], frameID)
	return buildRDPGFXPDU(rdpgfxCmdStartFrame, 0, payload)
}

func buildRDPGFXEndFramePDU(frameID uint32) []byte {
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload[0:4], frameID)
	return buildRDPGFXPDU(rdpgfxCmdEndFrame, 0, payload)
}

func buildRDPGFXWireToSurface1PDU(surfaceID uint16, codecID uint16, pixelFormat byte, destLeft, destTop, destRight, destBottom uint16, bitmapData []byte) []byte {
	payload := make([]byte, 0, 17+len(bitmapData))
	payload = appendLE16Bytes(payload, surfaceID)
	payload = appendLE16Bytes(payload, codecID)
	payload = append(payload, pixelFormat)
	payload = appendLE16Bytes(payload, destLeft)
	payload = appendLE16Bytes(payload, destTop)
	payload = appendLE16Bytes(payload, destRight)
	payload = appendLE16Bytes(payload, destBottom)
	payload = appendLE32Bytes(payload, uint32(len(bitmapData))) // #nosec G115 -- payload length is bounded by allocation.
	payload = append(payload, bitmapData...)
	return buildRDPGFXPDU(rdpgfxCmdWireToSurface1, 0, payload)
}

func buildRDPGFXH264FramePDUs(surfaceID uint16, frameID uint32, unit h264AccessUnit, width, height int) ([][]byte, bool) {
	if width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return nil, false
	}
	if err := validateH264AccessUnit(unit); err != nil {
		return nil, false
	}
	avc420Payload := buildRDPGFXAVC420BitmapStream(unit.Data, uint16(width), uint16(height)) // #nosec G115 -- dimensions bounded above.
	pdus := [][]byte{
		buildRDPGFXStartFramePDU(frameID),
		buildRDPGFXWireToSurface1PDU(surfaceID, rdpgfxCodecAVC420, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(width), uint16(height), avc420Payload), // #nosec G115 -- dimensions bounded above.
		buildRDPGFXEndFramePDU(frameID),
	}
	return pdus, true
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
	planar, ok := buildPlanarRLEPayload(normalized, stride)
	if !ok {
		return nil, false
	}
	return [][]byte{
		buildRDPGFXStartFramePDU(frameID),
		buildRDPGFXWireToSurface1PDU(surfaceID, rdpgfxCodecPlanar, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(normalized.Width), uint16(normalized.Height), planar), // #nosec G115 -- dimensions validated by normalizedFrameStride and desktop clamp.
		buildRDPGFXEndFramePDU(frameID),
	}, true
}

func buildPlanarRLEPayload(src frame.Frame, stride int) ([]byte, bool) {
	maxInt := int(^uint(0) >> 1)
	if src.Width <= 0 || src.Height <= 0 || src.Width > maxInt/src.Height {
		return nil, false
	}
	planeSize := src.Width * src.Height
	red := make([]byte, planeSize)
	green := make([]byte, planeSize)
	blue := make([]byte, planeSize)
	for y := 0; y < src.Height; y++ {
		for x := 0; x < src.Width; x++ {
			si := y*stride + x*4
			di := y*src.Width + x
			switch src.Format {
			case frame.PixelFormatBGRA8888:
				blue[di] = src.Data[si+0]
				green[di] = src.Data[si+1]
				red[di] = src.Data[si+2]
			case frame.PixelFormatRGBA8888:
				red[di] = src.Data[si+0]
				green[di] = src.Data[si+1]
				blue[di] = src.Data[si+2]
			default:
				return nil, false
			}
		}
	}
	out := []byte{0x30} // PLANAR_FORMAT_HEADER_NA | PLANAR_FORMAT_HEADER_RLE.
	out = append(out, encodePlanarDeltaRLEPlane(red, src.Width, src.Height)...)
	out = append(out, encodePlanarDeltaRLEPlane(green, src.Width, src.Height)...)
	out = append(out, encodePlanarDeltaRLEPlane(blue, src.Width, src.Height)...)
	return out, true
}

func encodePlanarDeltaRLEPlane(plane []byte, width, height int) []byte {
	out := make([]byte, 0, len(plane)/2)
	for y := 0; y < height; y++ {
		row := make([]byte, width)
		copy(row, plane[y*width:y*width+width])
		if y > 0 {
			prev := plane[(y-1)*width : (y-1)*width+width]
			for x := 0; x < width; x++ {
				row[x] = planarDeltaByte(int(row[x]) - int(prev[x]))
			}
		}
		out = appendPlanarRLELine(out, row)
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
	out := make([]byte, 8, 8+len(payload))
	binary.LittleEndian.PutUint16(out[0:2], cmdID)
	binary.LittleEndian.PutUint16(out[2:4], flags)
	binary.LittleEndian.PutUint32(out[4:8], uint32(8+len(payload))) // #nosec G115 -- payload length is bounded by allocation.
	out = append(out, payload...)
	return out
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
