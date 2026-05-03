package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestParseClientInfo(t *testing.T) {
	payload := buildClientInfoPayload("rui", "secret", "home")
	info, err := parseClientInfo(payload)
	if err != nil {
		t.Fatal(err)
	}
	if info.UserName != "rui" || info.Password != "secret" || info.Domain != "home" {
		t.Fatalf("unexpected client info: %#v", info)
	}
}

func TestParseClientInfoWithExternalTerminators(t *testing.T) {
	fields := [][]byte{
		encodeClientInfoStringWithoutTerminator(""),
		encodeClientInfoStringWithoutTerminator("runner"),
		encodeClientInfoStringWithoutTerminator("secret"),
		encodeClientInfoStringWithoutTerminator(""),
		encodeClientInfoStringWithoutTerminator(""),
	}
	payload := make([]byte, 18)
	binary.LittleEndian.PutUint32(payload[4:8], 0x00000010)
	for i, field := range fields {
		binary.LittleEndian.PutUint16(payload[8+i*2:10+i*2], uint16(len(field)))
	}
	for _, field := range fields {
		payload = append(payload, field...)
		payload = append(payload, 0, 0)
	}
	info, err := parseClientInfo(payload)
	if err != nil {
		t.Fatal(err)
	}
	if info.UserName != "runner" || info.Password != "secret" || info.Domain != "" {
		t.Fatalf("unexpected client info: %#v", info)
	}
}

func encodeClientInfoStringWithoutTerminator(s string) []byte {
	withTerminator := encodeClientInfoString(s)
	if len(withTerminator) < 2 {
		return nil
	}
	return withTerminator[:len(withTerminator)-2]
}

func TestStaticCredentials(t *testing.T) {
	auth := StaticCredentials{Username: "rui", Password: "secret"}
	if err := authenticateClientInfo(auth, ClientInfo{UserName: "rui", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := authenticateClientInfo(auth, ClientInfo{UserName: "rui", Password: "wrong"}); err == nil {
		t.Fatal("expected invalid credentials")
	}
}
