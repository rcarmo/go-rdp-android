package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:3390", "RDP server address")
	flag.Parse()

	conn, err := net.DialTimeout("tcp", *addr, 3*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := sendX224ConnectionRequest(conn); err != nil {
		log.Fatal(err)
	}
	resp, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("X.224 confirm: %x\n", resp)

	if err := sendMCSConnectInitial(conn); err != nil {
		log.Fatal(err)
	}
	mcsResp, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("MCS response: %x\n", mcsResp)

	if err := sendMCSDomainPDU(conn, 1, []byte{1, 0, 1, 0}); err != nil {
		log.Fatal(err)
	}
	fmt.Println("sent ErectDomainRequest")

	if err := sendMCSDomainPDU(conn, 10, nil); err != nil {
		log.Fatal(err)
	}
	attachResp, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("AttachUserConfirm: %x\n", attachResp)

	joinBody := append(encodePERInteger16(1001, 1001), encodePERInteger16(1003, 0)...)
	if err := sendMCSDomainPDU(conn, 14, joinBody); err != nil {
		log.Fatal(err)
	}
	joinResp, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ChannelJoinConfirm: %x\n", joinResp)

	clientInfo := []byte{0x40, 0, 0, 0, 1, 2, 3, 4}
	if err := sendMCSDomainPDU(conn, 25, buildMCSSendDataRequest(1001, 1003, clientInfo)); err != nil {
		log.Fatal(err)
	}
	fmt.Println("sent minimal Client Info security PDU")
	demand, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("DemandActive: %x\n", demand)

	confirm := buildConfirmActivePDU(0x000103ea, 1001)
	if err := sendMCSDomainPDU(conn, 25, buildMCSSendDataRequest(1001, 1003, confirm)); err != nil {
		log.Fatal(err)
	}
	fmt.Println("sent minimal Confirm Active PDU")

	if err := sendShareData(conn, 0x1f, synchronizePayload()); err != nil {
		log.Fatal(err)
	}
	readAndPrint(conn, "Server Synchronize")
	if err := sendShareData(conn, 0x14, controlPayload(0x0004)); err != nil {
		log.Fatal(err)
	}
	readAndPrint(conn, "Server Control Cooperate")
	if err := sendShareData(conn, 0x14, controlPayload(0x0001)); err != nil {
		log.Fatal(err)
	}
	readAndPrint(conn, "Server Control Granted")
	if err := sendShareData(conn, 0x27, []byte{0, 0, 0, 0, 3, 0, 0x32, 0}); err != nil {
		log.Fatal(err)
	}
	readAndPrint(conn, "Server FontMap")
}

func readTPKT(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint16(header[2:4]))
	payload := make([]byte, length-4)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

func writeTPKT(w io.Writer, payload []byte) error {
	header := []byte{3, 0, 0, 0}
	binary.BigEndian.PutUint16(header[2:4], uint16(4+len(payload)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func sendX224ConnectionRequest(conn net.Conn) error {
	neg := make([]byte, 8)
	neg[0] = 0x01 // RDP_NEG_REQ
	binary.LittleEndian.PutUint16(neg[2:4], 8)
	binary.LittleEndian.PutUint32(neg[4:8], 0x00000001) // SSL requested
	userData := append([]byte("Cookie: mstshash=probe\r\n"), neg...)
	li := byte(6 + len(userData))
	x224 := []byte{li, 0xe0, 0, 0, 0, 1, 0}
	x224 = append(x224, userData...)
	return writeTPKT(conn, x224)
}

func sendMCSConnectInitial(conn net.Conn) error {
	return writeTPKT(conn, []byte{0x02, 0xf0, 0x80, 0x7f, 0x65, 0x00})
}

func sendMCSDomainPDU(conn net.Conn, application int, body []byte) error {
	mcs := append([]byte{byte(application << 2)}, body...)
	return writeTPKT(conn, append([]byte{0x02, 0xf0, 0x80}, mcs...))
}

func encodePERInteger16(value, minimum uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, value-minimum)
	return buf
}

func buildMCSSendDataRequest(initiator, channelID uint16, data []byte) []byte {
	body := encodePERInteger16(initiator, 1001)
	body = append(body, encodePERInteger16(channelID, 0)...)
	body = append(body, 0x70)
	body = append(body, encodePERLength(len(data))...)
	body = append(body, data...)
	return body
}

func encodePERLength(length int) []byte {
	if length > 0x7f {
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(length)|0x8000)
		return buf
	}
	return []byte{byte(length)}
}

func readAndPrint(conn net.Conn, label string) {
	pkt, err := readTPKT(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %x\n", label, pkt)
}

func sendShareData(conn net.Conn, pduType2 byte, payload []byte) error {
	pdu := buildShareDataPDU(pduType2, payload)
	return sendMCSDomainPDU(conn, 25, buildMCSSendDataRequest(1001, 1003, pdu))
}

func buildShareDataPDU(pduType2 byte, payload []byte) []byte {
	totalLength := 18 + len(payload)
	out := appendLE16(nil, uint16(totalLength))
	out = appendLE16(out, 0x0017)
	out = appendLE16(out, 1001)
	out = appendLE32(out, 0x000103ea)
	out = append(out, 0, 1)
	out = appendLE16(out, uint16(4+len(payload)))
	out = append(out, pduType2, 0)
	out = appendLE16(out, 0)
	out = append(out, payload...)
	return out
}

func synchronizePayload() []byte {
	out := appendLE16(nil, 1)
	out = appendLE16(out, 1002)
	return out
}

func controlPayload(action uint16) []byte {
	out := appendLE16(nil, action)
	out = appendLE16(out, 0)
	out = appendLE32(out, 0)
	return out
}

func buildConfirmActivePDU(shareID uint32, userID uint16) []byte {
	source := []byte("PROBE")
	cap := capabilitySet(0x0001, generalCapability())
	combinedCapsLen := 4 + len(cap)
	totalLength := 6 + 4 + 2 + 2 + 2 + len(source) + combinedCapsLen
	pdu := appendLE16(nil, uint16(totalLength))
	pdu = appendLE16(pdu, 0x0013)
	pdu = appendLE16(pdu, userID)
	pdu = appendLE32(pdu, shareID)
	pdu = appendLE16(pdu, 1002)
	pdu = appendLE16(pdu, uint16(len(source)))
	pdu = appendLE16(pdu, uint16(combinedCapsLen))
	pdu = append(pdu, source...)
	pdu = appendLE16(pdu, 1)
	pdu = appendLE16(pdu, 0)
	pdu = append(pdu, cap...)
	return pdu
}

func capabilitySet(capType uint16, payload []byte) []byte {
	out := appendLE16(nil, capType)
	out = appendLE16(out, uint16(4+len(payload)))
	out = append(out, payload...)
	return out
}

func generalCapability() []byte {
	out := make([]byte, 0, 22)
	for _, v := range []uint16{1, 3, 0x0200, 0, 0, 0, 0, 0, 0, 0, 0} {
		out = appendLE16(out, v)
	}
	return out
}

func appendLE16(dst []byte, v uint16) []byte {
	return append(dst, byte(v), byte(v>>8))
}

func appendLE32(dst []byte, v uint32) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
