package rdpserver

import (
	"bytes"
	"encoding/binary"
	"net"
)

const (
	secLicensePacket = 0x0080

	licenseErrorAlert        = 0xff
	licensePreambleVersion3  = 0x03
	licenseStatusValidClient = 0x00000007
	licenseStateNoTransition = 0x00000002
	licenseBlobError         = 0x0004
)

func writeLicenseValidClient(conn net.Conn) error {
	pdu := buildLicenseValidClientPDU()
	body := buildMCSSendDataIndication(serverChannelID, globalChannelID, pdu)
	return writeMCSDomainPDU(conn, mcsSendDataIndicationApp, body)
}

func buildLicenseValidClientPDU() []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint16(secLicensePacket))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(buf, binary.LittleEndian, uint8(licenseErrorAlert))
	_ = binary.Write(buf, binary.LittleEndian, uint8(licensePreambleVersion3))
	_ = binary.Write(buf, binary.LittleEndian, uint16(16))
	_ = binary.Write(buf, binary.LittleEndian, uint32(licenseStatusValidClient))
	_ = binary.Write(buf, binary.LittleEndian, uint32(licenseStateNoTransition))
	_ = binary.Write(buf, binary.LittleEndian, uint16(licenseBlobError))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	return buf.Bytes()
}
