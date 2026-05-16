package rdpserver

import (
	"encoding/binary"
	"fmt"
)

const (
	rdpgfxDynamicChannelName = "Microsoft::Windows::RDS::Graphics"

	rdpgfxCmdCapsAdvertise uint16 = 0x0001
	rdpgfxCmdCapsConfirm   uint16 = 0x0002

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
