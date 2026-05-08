package rdpserver

import (
	"bytes"
	"encoding/binary"
	"strings"
)

const (
	gccUserDataCS_CORE    = 0xc001
	gccUserDataCS_NET     = 0xc003
	gccUserDataCS_MONITOR = 0xc005
	staticChannelBase     = 1004
)

type clientChannel struct {
	Name string
	ID   uint16
}

type clientDisplaySettings struct {
	DesktopWidth         uint16
	DesktopHeight        uint16
	CoreDesktopPresent   bool
	MonitorLayoutPresent bool
	MonitorCount         uint32
}

func parseClientChannelsFromConnectInitial(data []byte) []clientChannel {
	payload, ok := findClientUserDataBlockPayload(data, gccUserDataCS_NET)
	if !ok || len(payload) < 4 {
		return nil
	}
	count := int(binary.LittleEndian.Uint32(payload[0:4]))
	if count < 0 || count > 64 || 4+count*12 > len(payload) {
		return nil
	}
	channels := make([]clientChannel, 0, count)
	pos := 4
	for i := 0; i < count; i++ {
		nameBytes := payload[pos : pos+8]
		name := strings.TrimRight(string(bytes.TrimRight(nameBytes, "\x00")), " ")
		channels = append(channels, clientChannel{Name: name, ID: uint16(staticChannelBase + i)})
		pos += 12 // name[8] + options[4]
	}
	return channels
}

func parseClientDisplaySettingsFromConnectInitial(data []byte) clientDisplaySettings {
	settings := clientDisplaySettings{}

	if core, ok := findClientUserDataBlockPayload(data, gccUserDataCS_CORE); ok && len(core) >= 8 {
		settings.CoreDesktopPresent = true
		settings.DesktopWidth = binary.LittleEndian.Uint16(core[4:6])
		settings.DesktopHeight = binary.LittleEndian.Uint16(core[6:8])
	}
	if monitor, ok := findClientUserDataBlockPayload(data, gccUserDataCS_MONITOR); ok && len(monitor) >= 8 {
		settings.MonitorLayoutPresent = true
		settings.MonitorCount = binary.LittleEndian.Uint32(monitor[4:8])
	}
	return settings
}

func findClientUserDataBlockPayload(data []byte, blockType uint16) ([]byte, bool) {
	for off := 0; off+4 <= len(data); off++ {
		if binary.LittleEndian.Uint16(data[off:off+2]) != blockType {
			continue
		}
		blockLen := int(binary.LittleEndian.Uint16(data[off+2 : off+4]))
		if blockLen < 4 || off+blockLen > len(data) {
			continue
		}
		return data[off+4 : off+blockLen], true
	}
	return nil, false
}
