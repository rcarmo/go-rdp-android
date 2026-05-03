package rdpserver

import (
	"bytes"
	"encoding/binary"
	"strings"
)

const (
	gccUserDataCS_NET = 0xc003
	staticChannelBase = 1004
)

type clientChannel struct {
	Name string
	ID   uint16
}

func parseClientChannelsFromConnectInitial(data []byte) []clientChannel {
	for off := 0; off+8 <= len(data); off++ {
		if binary.LittleEndian.Uint16(data[off:off+2]) != gccUserDataCS_NET {
			continue
		}
		blockLen := int(binary.LittleEndian.Uint16(data[off+2 : off+4]))
		if blockLen < 8 || off+blockLen > len(data) {
			continue
		}
		count := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if count < 0 || count > 64 || 8+count*12 > blockLen {
			continue
		}
		channels := make([]clientChannel, 0, count)
		pos := off + 8
		for i := 0; i < count; i++ {
			nameBytes := data[pos : pos+8]
			name := strings.TrimRight(string(bytes.TrimRight(nameBytes, "\x00")), " ")
			channels = append(channels, clientChannel{Name: name, ID: uint16(staticChannelBase + i)})
			pos += 12 // name[8] + options[4]
		}
		return channels
	}
	return nil
}
