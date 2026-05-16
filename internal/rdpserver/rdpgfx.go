package rdpserver

import (
	"encoding/binary"
	"fmt"

	"github.com/rcarmo/go-rdp-android/internal/frame"
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

	rdpgfxCodecUncompressed uint16 = 0x0000
	rdpgfxCodecClearCodec   uint16 = 0x0008
	rdpgfxCodecPlanar       uint16 = 0x000a

	rdpgfxPixelFormatXRGB8888 byte = 0x20

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
	Version uint32
	Flags   uint32
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
	if len(data) < count*8 {
		return nil, fmt.Errorf("short RDPGFX capability sets: got %d need %d", len(data), count*8)
	}
	caps := make([]rdpgfxCapabilitySet, 0, count)
	for i := 0; i < count; i++ {
		caps = append(caps, rdpgfxCapabilitySet{
			Version: binary.LittleEndian.Uint32(data[i*8 : i*8+4]),
			Flags:   binary.LittleEndian.Uint32(data[i*8+4 : i*8+8]),
		})
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

func supportedRDPGFXVersion(version uint32) bool {
	switch version {
	case rdpgfxCapsVersion8, rdpgfxCapsVersion81, rdpgfxCapsVersion10, rdpgfxCapsVersion102, rdpgfxCapsVersion103, rdpgfxCapsVersion104, rdpgfxCapsVersion105, rdpgfxCapsVersion106:
		return true
	default:
		return false
	}
}

func buildRDPGFXCapsConfirmPDU(cap rdpgfxCapabilitySet) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload[0:4], cap.Version)
	binary.LittleEndian.PutUint32(payload[4:8], cap.Flags)
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
	payload := make([]byte, 6)
	binary.LittleEndian.PutUint16(payload[0:2], surfaceID)
	binary.LittleEndian.PutUint16(payload[2:4], uint16(originX)) // #nosec G115 -- bounded above.
	binary.LittleEndian.PutUint16(payload[4:6], uint16(originY)) // #nosec G115 -- bounded above.
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

func buildRDPGFXUncompressedFramePDUs(surfaceID uint16, frameID uint32, src frame.Frame, width, height int) ([][]byte, bool) {
	normalized := normalizeFrameForDesktop(src, width, height)
	stride, ok := normalizedFrameStride(normalized)
	if !ok || normalized.Format != frame.PixelFormatRGBA8888 && normalized.Format != frame.PixelFormatBGRA8888 {
		return nil, false
	}
	maxInt := int(^uint(0) >> 1)
	if normalized.Width > maxInt/4 || normalized.Height > maxInt/(normalized.Width*4) {
		return nil, false
	}
	pixels := make([]byte, normalized.Width*normalized.Height*4)
	for y := 0; y < normalized.Height; y++ {
		for x := 0; x < normalized.Width; x++ {
			si := y*stride + x*4
			di := (y*normalized.Width + x) * 4
			switch normalized.Format {
			case frame.PixelFormatBGRA8888:
				pixels[di+0] = normalized.Data[si+0]
				pixels[di+1] = normalized.Data[si+1]
				pixels[di+2] = normalized.Data[si+2]
				pixels[di+3] = 0x00
			case frame.PixelFormatRGBA8888:
				pixels[di+0] = normalized.Data[si+2]
				pixels[di+1] = normalized.Data[si+1]
				pixels[di+2] = normalized.Data[si+0]
				pixels[di+3] = 0x00
			}
		}
	}
	return [][]byte{
		buildRDPGFXStartFramePDU(frameID),
		buildRDPGFXWireToSurface1PDU(surfaceID, rdpgfxCodecUncompressed, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(normalized.Width-1), uint16(normalized.Height-1), pixels), // #nosec G115 -- dimensions validated by normalizedFrameStride and desktop clamp.
		buildRDPGFXEndFramePDU(frameID),
	}, true
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
			tracef("rdpgfx_cap", "index=%d version=0x%08x flags=0x%08x supported=%t", i, cap.Version, cap.Flags, supportedRDPGFXVersion(cap.Version))
		}
	default:
		tracef("rdpgfx_pdu", "cmd=0x%04x flags=0x%04x length=%d", pdu.CmdID, pdu.Flags, pdu.Length)
	}
}
