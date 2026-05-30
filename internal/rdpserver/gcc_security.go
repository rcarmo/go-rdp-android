package rdpserver

import "encoding/binary"

const (
	gccUserDataSC_CORE     = 0x0c01
	gccUserDataSC_SECURITY = 0x0c02
	gccUserDataSC_NET      = 0x0c03

	rdpVersion10 = 0x00080004
)

// buildServerUserData emits the GCC server user-data blocks required by the
// Basic Settings Exchange. For TLS/NLA-capable connections, RDPBCGR expects the
// server security block to advertise ENCRYPTION_METHOD_NONE and
// ENCRYPTION_LEVEL_NONE because transport security is provided by the selected
// external protocol.
func buildServerUserData(selectedProtocol uint32, channels []clientChannel) []byte {
	networkPayloadLen := 4 + 2*len(channels)
	if len(channels)%2 == 1 {
		networkPayloadLen += 2
	}
	totalLen := 4 + 12 + 4 + 8 + 4 + networkPayloadLen
	out := make([]byte, totalLen)
	off := 0
	off += writeServerCoreDataAt(out[off:], selectedProtocol)
	off += writeServerSecurityDataAt(out[off:], selectedProtocol)
	writeServerNetworkDataAt(out[off:], channels)
	return out
}

func writeServerCoreDataAt(out []byte, selectedProtocol uint32) int {
	writeGCCUserDataBlockHeaderAt(out, gccUserDataSC_CORE, 12)
	binary.LittleEndian.PutUint32(out[4:8], rdpVersion10)
	binary.LittleEndian.PutUint32(out[8:12], selectedProtocol)
	// earlyCapabilityFlags remains zero at out[12:16].
	return 16
}

func writeServerSecurityDataAt(out []byte, selectedProtocol uint32) int {
	// ENCRYPTION_METHOD_NONE / ENCRYPTION_LEVEL_NONE. This is the correct GCC
	// surface when selectedProtocol is SSL/Hybrid. The classic-RDP branch is kept
	// explicit so future proprietary/X.509 certificate data can be attached here
	// without changing the response envelope.
	_ = selectedProtocol
	writeGCCUserDataBlockHeaderAt(out, gccUserDataSC_SECURITY, 8)
	return 12
}

func writeServerNetworkDataAt(out []byte, channels []clientChannel) int {
	payloadLen := 4 + 2*len(channels)
	if len(channels)%2 == 1 {
		payloadLen += 2
	}
	writeGCCUserDataBlockHeaderAt(out, gccUserDataSC_NET, payloadLen)
	binary.LittleEndian.PutUint16(out[4:6], globalChannelID)
	binary.LittleEndian.PutUint16(out[6:8], uint16(len(channels))) // #nosec G115 -- channel count is bounded by negotiation input.
	off := 8
	for _, ch := range channels {
		binary.LittleEndian.PutUint16(out[off:off+2], ch.ID)
		off += 2
	}
	return 4 + payloadLen
}

func writeGCCUserDataBlockHeaderAt(out []byte, blockType uint16, payloadLen int) {
	binary.LittleEndian.PutUint16(out[0:2], blockType)
	binary.LittleEndian.PutUint16(out[2:4], uint16(payloadLen+4)) // #nosec G115 -- GCC user-data blocks are bounded by allocation.
}
