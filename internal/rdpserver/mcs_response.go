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

var t12402098OID = [6]byte{0, 0, 20, 124, 0, 1}

func writeMCSConnectResponse(conn net.Conn, selectedProtocol uint32, channels []clientChannel) error {
	gcc := buildGCCConferenceCreateResponse(buildServerUserData(selectedProtocol, channels))
	body := new(bytes.Buffer)
	berWriteEnumerated(mcsResultSuccessful, body)
	berWriteInteger(1001, body) // calledConnectId
	berWriteSequence(defaultDomainParameters().serialize(), body)
	berWriteOctetString(gcc, body)

	mcs := new(bytes.Buffer)
	berWriteApplicationTag(mcsConnectResponseAppTag, body.Len(), mcs)
	mcs.Write(body.Bytes())

	x224 := append([]byte{0x02, x224TypeData, 0x80}, mcs.Bytes()...)
	return writeTPKT(conn, x224)
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
	inner := new(bytes.Buffer)
	perWriteChoice(0, inner)
	perWriteInteger16(1001, inner)
	perWriteInteger(1, inner)
	perWriteEnumerates(0, inner)
	perWriteNumberOfSet(1, inner)
	perWriteChoice(0xc0, inner)
	perWriteOctetStream("McDn", 4, inner)
	perWriteLength(len(serverUserData), inner)
	inner.Write(serverUserData)

	buf := new(bytes.Buffer)
	perWriteChoice(0, buf)
	perWriteObjectIdentifier(t12402098OID, buf)
	perWriteLength(inner.Len(), buf)
	buf.Write(inner.Bytes())
	return buf.Bytes()
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
