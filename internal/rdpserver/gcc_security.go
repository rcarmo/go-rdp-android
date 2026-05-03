package rdpserver

import (
	"bytes"
	"encoding/binary"
)

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
	buf := new(bytes.Buffer)
	writeServerCoreData(buf, selectedProtocol)
	writeServerSecurityData(buf, selectedProtocol)
	writeServerNetworkData(buf, channels)
	return buf.Bytes()
}

func writeServerCoreData(buf *bytes.Buffer, selectedProtocol uint32) {
	payload := new(bytes.Buffer)
	_ = binary.Write(payload, binary.LittleEndian, uint32(rdpVersion10))
	_ = binary.Write(payload, binary.LittleEndian, selectedProtocol)
	_ = binary.Write(payload, binary.LittleEndian, uint32(0)) // earlyCapabilityFlags
	writeGCCUserDataBlock(buf, gccUserDataSC_CORE, payload.Bytes())
}

func writeServerSecurityData(buf *bytes.Buffer, selectedProtocol uint32) {
	payload := new(bytes.Buffer)
	// ENCRYPTION_METHOD_NONE / ENCRYPTION_LEVEL_NONE. This is the correct GCC
	// surface when selectedProtocol is SSL/Hybrid. The classic-RDP branch is kept
	// explicit so future proprietary/X.509 certificate data can be attached here
	// without changing the response envelope.
	_ = selectedProtocol
	_ = binary.Write(payload, binary.LittleEndian, uint32(0))
	_ = binary.Write(payload, binary.LittleEndian, uint32(0))
	writeGCCUserDataBlock(buf, gccUserDataSC_SECURITY, payload.Bytes())
}

func writeServerNetworkData(buf *bytes.Buffer, channels []clientChannel) {
	payload := new(bytes.Buffer)
	_ = binary.Write(payload, binary.LittleEndian, uint16(globalChannelID))
	_ = binary.Write(payload, binary.LittleEndian, uint16(len(channels)))
	for _, ch := range channels {
		_ = binary.Write(payload, binary.LittleEndian, ch.ID)
	}
	if len(channels)%2 == 1 {
		_ = binary.Write(payload, binary.LittleEndian, uint16(0))
	}
	writeGCCUserDataBlock(buf, gccUserDataSC_NET, payload.Bytes())
}

func writeGCCUserDataBlock(buf *bytes.Buffer, blockType uint16, payload []byte) {
	_ = binary.Write(buf, binary.LittleEndian, blockType)
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(payload)+4))
	_, _ = buf.Write(payload)
}
