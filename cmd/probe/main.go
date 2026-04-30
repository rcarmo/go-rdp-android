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
