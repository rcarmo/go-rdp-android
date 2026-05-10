package rdpserver

import (
	"context"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// Authenticator validates parsed RDP Client Info credentials.
type Authenticator interface {
	Authenticate(ctx context.Context, info ClientInfo) error
}

// StaticCredentials is a simple username/password authenticator for the MVP.
type StaticCredentials struct {
	Username string
	Password string
}

// Authenticate validates username/password using constant-time comparison.
func (a StaticCredentials) Authenticate(_ context.Context, info ClientInfo) error {
	if a.Username == "" && a.Password == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(info.UserName), []byte(a.Username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(info.Password), []byte(a.Password)) != 1 {
		return fmt.Errorf("invalid credentials for user %q", sanitizeForLog(info.UserName, 64))
	}
	return nil
}

// ClientInfo captures fields from the RDP Client Info PDU.
type ClientInfo struct {
	CodePage       uint32
	Flags          uint32
	Domain         string
	UserName       string
	Password       string
	AlternateShell string
	WorkingDir     string
}

func authenticateClientInfo(auth Authenticator, info ClientInfo) error {
	if auth == nil {
		return nil
	}
	return auth.Authenticate(context.Background(), info)
}

func parseClientInfo(payload []byte) (ClientInfo, error) {
	var info ClientInfo
	if len(payload) < 18 {
		return info, fmt.Errorf("short Client Info PDU")
	}
	info.CodePage = binary.LittleEndian.Uint32(payload[0:4])
	info.Flags = binary.LittleEndian.Uint32(payload[4:8])
	lengths := []int{
		int(binary.LittleEndian.Uint16(payload[8:10])),
		int(binary.LittleEndian.Uint16(payload[10:12])),
		int(binary.LittleEndian.Uint16(payload[12:14])),
		int(binary.LittleEndian.Uint16(payload[14:16])),
		int(binary.LittleEndian.Uint16(payload[16:18])),
	}
	off := 18
	fields := make([]string, len(lengths))
	unicodeStrings := info.Flags&0x00000010 != 0
	for i, n := range lengths {
		if n < 0 || off+n > len(payload) {
			return info, fmt.Errorf("Client Info field %d length %d exceeds available %d", i, n, len(payload)-off)
		}
		fields[i] = decodeClientInfoString(payload[off : off+n])
		off += n
		off = skipClientInfoTerminator(payload, off, n, unicodeStrings)
	}
	info.Domain = fields[0]
	info.UserName = fields[1]
	info.Password = fields[2]
	info.AlternateShell = fields[3]
	info.WorkingDir = fields[4]
	return info, nil
}

func skipClientInfoTerminator(payload []byte, off, fieldLength int, unicodeStrings bool) int {
	if unicodeStrings {
		if fieldLength >= 2 && off >= 2 && payload[off-2] == 0 && payload[off-1] == 0 {
			return off
		}
		if len(payload)-off >= 2 && payload[off] == 0 && payload[off+1] == 0 {
			return off + 2
		}
		return off
	}
	if fieldLength >= 1 && off >= 1 && payload[off-1] == 0 {
		return off
	}
	if len(payload)-off >= 1 && payload[off] == 0 {
		return off + 1
	}
	return off
}

func decodeClientInfoString(data []byte) string {
	for len(data) >= 2 && data[len(data)-1] == 0 && data[len(data)-2] == 0 {
		data = data[:len(data)-2]
	}
	if len(data) == 0 {
		return ""
	}
	if len(data)%2 != 0 {
		return strings.TrimRight(string(data), "\x00")
	}
	u16 := make([]uint16, len(data)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}
	return string(utf16.Decode(u16))
}

func buildClientInfoPayload(username, password, domain string) []byte {
	fields := [][]byte{
		encodeClientInfoString(domain),
		encodeClientInfoString(username),
		encodeClientInfoString(password),
		encodeClientInfoString(""),
		encodeClientInfoString(""),
	}
	out := make([]byte, 18)
	binary.LittleEndian.PutUint32(out[0:4], 0)          // codePage
	binary.LittleEndian.PutUint32(out[4:8], 0x00000010) // INFO_UNICODE
	for i, field := range fields {
		binary.LittleEndian.PutUint16(out[8+i*2:10+i*2], uint16(len(field)))
	}
	for _, field := range fields {
		out = append(out, field...)
	}
	return out
}

func encodeClientInfoString(s string) []byte {
	runes := utf16.Encode([]rune(s + "\x00"))
	out := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], r)
	}
	return out
}
