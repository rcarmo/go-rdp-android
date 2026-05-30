package rdpserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const (
	mcsConnectResponseAppTag = 102
	mcsResultSuccessful      = 0
)

var (
	t12402098OID = [6]byte{0, 0, 20, 124, 0, 1}

	defaultDomainParametersBER = [...]byte{
		0x02, 0x01, 34, // maxChannelIds
		0x02, 0x01, 2, // maxUserIds
		0x02, 0x01, 0, // maxTokenIds
		0x02, 0x01, 1, // numPriorities
		0x02, 0x01, 0, // minThroughput
		0x02, 0x01, 1, // maxHeight
		0x02, 0x02, 0xff, 0xff, // maxMCSPDUSize
		0x02, 0x01, 2, // protocolVersion
	}
)

func writeMCSConnectResponse(conn net.Conn, selectedProtocol uint32, channels []clientChannel) error {
	userDataLen := serverUserDataLen(channels)
	gccLen := gccConferenceCreateResponseLen(userDataLen)
	params := defaultDomainParameters().serialize()
	bodyLen := 3 + 4 + 1 + berLengthSize(len(params)) + len(params) + 1 + berLengthSize(gccLen) + gccLen
	mcsLen := 2 + berLengthSize(bodyLen) + bodyLen
	totalLen := 4 + 3 + mcsLen
	if totalLen > 0xffff {
		return fmt.Errorf("MCS Connect Response too large: %d", mcsLen)
	}
	out := make([]byte, totalLen)
	out[0] = tpktVersion
	binary.BigEndian.PutUint16(out[2:4], uint16(totalLen))
	out[4] = 0x02
	out[5] = x224TypeData
	out[6] = 0x80
	off := 7
	out[off] = 0x7f
	out[off+1] = byte(mcsConnectResponseAppTag)
	off += 2
	off += writeBERLength(out[off:], bodyLen)
	out[off] = 0x0a // ENUMERATED
	out[off+1] = 1
	out[off+2] = mcsResultSuccessful
	off += 3
	out[off] = 0x02 // INTEGER calledConnectId
	out[off+1] = 2
	binary.BigEndian.PutUint16(out[off+2:off+4], 1001)
	off += 4
	out[off] = 0x30 // SEQUENCE domain parameters
	off++
	off += writeBERLength(out[off:], len(params))
	off += copy(out[off:], params)
	out[off] = 0x04 // OCTET STRING GCC Conference Create Response
	off++
	off += writeBERLength(out[off:], gccLen)
	writeGCCConferenceCreateResponseAt(out[off:off+gccLen], userDataLen, func(userDataOut []byte) {
		writeServerUserDataAt(userDataOut, selectedProtocol, channels)
	})
	_, err := conn.Write(out)
	if err == nil && traceEnabled {
		tracef("tpkt_write", "payload_len=%d", totalLen-4)
	}
	return err
}

func berLengthSize(size int) int {
	if size > 0xff {
		return 3
	}
	if size > 0x7f {
		return 2
	}
	return 1
}

func writeBERLength(out []byte, size int) int {
	if size > 0xff {
		out[0] = 0x82
		binary.BigEndian.PutUint16(out[1:3], uint16(size)) // #nosec G115 -- BER payloads are bounded by allocation.
		return 3
	}
	if size > 0x7f {
		out[0] = 0x81
		out[1] = byte(size)
		return 2
	}
	out[0] = byte(size)
	return 1
}

type domainParameters struct {
	maxChannelIds   int
	maxUserIds      int
	maxTokenIds     int
	numPriorities   int
	minThroughput   int
	maxHeight       int
	maxMCSPDUSize   int
	protocolVersion int
}

func defaultDomainParameters() domainParameters {
	return domainParameters{
		maxChannelIds:   34,
		maxUserIds:      2,
		maxTokenIds:     0,
		numPriorities:   1,
		minThroughput:   0,
		maxHeight:       1,
		maxMCSPDUSize:   65535,
		protocolVersion: 2,
	}
}

func (p domainParameters) serialize() []byte {
	if p == defaultDomainParameters() {
		return defaultDomainParametersBER[:]
	}
	buf := new(bytes.Buffer)
	berWriteInteger(p.maxChannelIds, buf)
	berWriteInteger(p.maxUserIds, buf)
	berWriteInteger(p.maxTokenIds, buf)
	berWriteInteger(p.numPriorities, buf)
	berWriteInteger(p.minThroughput, buf)
	berWriteInteger(p.maxHeight, buf)
	berWriteInteger(p.maxMCSPDUSize, buf)
	berWriteInteger(p.protocolVersion, buf)
	return buf.Bytes()
}

func buildGCCConferenceCreateResponse(serverUserData []byte) []byte {
	out := make([]byte, gccConferenceCreateResponseLen(len(serverUserData)))
	writeGCCConferenceCreateResponseAt(out, len(serverUserData), func(userDataOut []byte) {
		copy(userDataOut, serverUserData)
	})
	return out
}

func gccConferenceCreateResponseLen(userDataLen int) int {
	innerLen := 13 + encodedPERLengthSize(userDataLen) + userDataLen
	return 1 + 6 + encodedPERLengthSize(innerLen) + innerLen
}

func writeGCCConferenceCreateResponseAt(out []byte, userDataLen int, writeUserData func([]byte)) {
	innerLen := 13 + encodedPERLengthSize(userDataLen) + userDataLen
	off := 0
	out[off] = 0 // choice
	off++
	off += writePERObjectIdentifier(out[off:], t12402098OID)
	off += writePERLength(out[off:], innerLen)
	out[off] = 0 // choice
	off++
	binary.BigEndian.PutUint16(out[off:off+2], 0) // calledConnectId: 1001 encoded relative to 1001.
	off += 2
	out[off] = 1 // PER length for integer(1)
	out[off+1] = 1
	off += 2
	out[off] = 0      // enumerates
	out[off+1] = 1    // numberOfSet
	out[off+2] = 0xc0 // choice
	off += 3
	out[off] = 0 // octet-stream length for "McDn" relative to min 4.
	copy(out[off+1:off+5], "McDn")
	off += 5
	off += writePERLength(out[off:], userDataLen)
	writeUserData(out[off : off+userDataLen])
}

func writePERObjectIdentifier(out []byte, oid [6]byte) int {
	out[0] = 5
	out[1] = (oid[0] << 4) | (oid[1] & 0x0f)
	copy(out[2:6], oid[2:])
	return 6
}

func berWriteApplicationTag(tag int, size int, w io.Writer) {
	if tag > 30 {
		_, _ = w.Write([]byte{0x7f, byte(tag)})
	} else {
		_, _ = w.Write([]byte{byte(0x60 | tag)})
	}
	berWriteLength(size, w)
}

func berWriteLength(size int, w io.Writer) {
	if size > 0xff {
		_, _ = w.Write([]byte{0x82})
		_ = binary.Write(w, binary.BigEndian, uint16(size))
	} else if size > 0x7f {
		_, _ = w.Write([]byte{0x81, byte(size)})
	} else {
		_, _ = w.Write([]byte{byte(size)})
	}
}

func berWriteEnumerated(value int, w io.Writer) {
	_, _ = w.Write([]byte{0x0a})
	berWriteLength(1, w)
	_, _ = w.Write([]byte{byte(value)})
}

func berWriteInteger(value int, w io.Writer) {
	_, _ = w.Write([]byte{0x02})
	if value <= 0xff {
		berWriteLength(1, w)
		_, _ = w.Write([]byte{byte(value)})
		return
	}
	if value <= 0xffff {
		berWriteLength(2, w)
		_ = binary.Write(w, binary.BigEndian, uint16(value))
		return
	}
	berWriteLength(4, w)
	_ = binary.Write(w, binary.BigEndian, uint32(value))
}

func berWriteSequence(data []byte, w io.Writer) {
	_, _ = w.Write([]byte{0x30})
	berWriteLength(len(data), w)
	_, _ = w.Write(data)
}

func berWriteOctetString(data []byte, w io.Writer) {
	_, _ = w.Write([]byte{0x04})
	berWriteLength(len(data), w)
	_, _ = w.Write(data)
}

func perWriteChoice(value byte, w io.Writer)      { _, _ = w.Write([]byte{value}) }
func perWriteEnumerates(value byte, w io.Writer)  { _, _ = w.Write([]byte{value}) }
func perWriteNumberOfSet(value byte, w io.Writer) { _, _ = w.Write([]byte{value}) }

func perWriteInteger16(value uint16, w io.Writer) {
	_ = binary.Write(w, binary.BigEndian, value-1001)
}

func perWriteInteger(value int, w io.Writer) {
	if value <= 0xff {
		perWriteLength(1, w)
		_, _ = w.Write([]byte{byte(value)})
		return
	}
	perWriteLength(2, w)
	_ = binary.Write(w, binary.BigEndian, uint16(value))
}

func perWriteObjectIdentifier(oid [6]byte, w io.Writer) {
	perWriteLength(5, w)
	_, _ = w.Write([]byte{(oid[0] << 4) | (oid[1] & 0x0f), oid[2], oid[3], oid[4], oid[5]})
}

func perWriteOctetStream(value string, minValue int, w io.Writer) {
	length := len(value) - minValue
	if length < 0 {
		length = 0
	}
	perWriteLength(length, w)
	_, _ = w.Write([]byte(value))
}

func perWriteLength(value int, w io.Writer) {
	if value > 0x7f {
		_ = binary.Write(w, binary.BigEndian, uint16(value)|0x8000)
		return
	}
	_, _ = w.Write([]byte{byte(value)})
}

func parseMCSConnectResponse(data []byte) (*MCSInfo, error) {
	r := bytes.NewReader(data)
	appTag, payloadLen, err := readBERApplicationTag(r)
	if err != nil {
		return nil, err
	}
	if appTag != mcsConnectResponseAppTag {
		return nil, fmt.Errorf("unexpected MCS application tag %d", appTag)
	}
	return &MCSInfo{ApplicationTag: appTag, PayloadLength: payloadLen, UserDataLength: r.Len()}, nil
}
