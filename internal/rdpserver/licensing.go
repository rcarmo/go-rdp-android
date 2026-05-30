package rdpserver

import (
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

var licenseValidClientPDU = [...]byte{
	byte(secLicensePacket), byte(secLicensePacket >> 8), 0, 0,
	licenseErrorAlert, licensePreambleVersion3, 16, 0,
	byte(licenseStatusValidClient), byte(licenseStatusValidClient >> 8), byte(licenseStatusValidClient >> 16), byte(licenseStatusValidClient >> 24),
	byte(licenseStateNoTransition), byte(licenseStateNoTransition >> 8), byte(licenseStateNoTransition >> 16), byte(licenseStateNoTransition >> 24),
	byte(licenseBlobError), byte(licenseBlobError >> 8), 0, 0,
}

func writeLicenseValidClient(conn net.Conn) error {
	pdu := buildLicenseValidClientPDU()
	dataLen := len(pdu)
	perLen := encodedPERLengthSize(dataLen)
	bodyLen := 2 + 2 + 1 + perLen + dataLen
	totalLen := 4 + 3 + 1 + bodyLen
	out := make([]byte, totalLen)
	writeTPKTX224MCSHeader(out, mcsSendDataIndicationApp, bodyLen)
	binary.BigEndian.PutUint16(out[8:10], serverChannelID-defaultMCSUserID)
	binary.BigEndian.PutUint16(out[10:12], globalChannelID)
	out[12] = 0x70
	dataOff := 13 + writePERLength(out[13:], dataLen)
	copy(out[dataOff:], pdu)
	_, err := conn.Write(out)
	if err == nil && traceEnabled {
		tracef("tpkt_write", "payload_len=%d", totalLen-4)
	}
	return err
}

func buildLicenseValidClientPDU() []byte {
	return licenseValidClientPDU[:]
}
