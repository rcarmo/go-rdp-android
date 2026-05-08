package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestParseClientChannelsFromConnectInitial(t *testing.T) {
	block := make([]byte, 8)
	binary.LittleEndian.PutUint16(block[0:2], gccUserDataCS_NET)
	binary.LittleEndian.PutUint32(block[4:8], 2)
	for _, name := range []string{"rdpdr", "cliprdr"} {
		def := make([]byte, 12)
		copy(def[:8], name)
		block = append(block, def...)
	}
	binary.LittleEndian.PutUint16(block[2:4], uint16(len(block)))
	data := append([]byte{0xaa, 0xbb, 0xcc}, block...)

	channels := parseClientChannelsFromConnectInitial(data)
	if len(channels) != 2 {
		t.Fatalf("channel count = %d, want 2", len(channels))
	}
	if channels[0].Name != "rdpdr" || channels[0].ID != staticChannelBase {
		t.Fatalf("unexpected first channel: %#v", channels[0])
	}
	if channels[1].Name != "cliprdr" || channels[1].ID != staticChannelBase+1 {
		t.Fatalf("unexpected second channel: %#v", channels[1])
	}
}

func TestParseClientDisplaySettingsFromConnectInitial(t *testing.T) {
	core := make([]byte, 8)
	binary.LittleEndian.PutUint32(core[0:4], 0x00080004)
	binary.LittleEndian.PutUint16(core[4:6], 1920)
	binary.LittleEndian.PutUint16(core[6:8], 1080)
	monitor := make([]byte, 8)
	binary.LittleEndian.PutUint32(monitor[0:4], 1)
	binary.LittleEndian.PutUint32(monitor[4:8], 2)

	data := append([]byte{0xaa, 0xbb}, appendClientUserDataBlockForTest(nil, gccUserDataCS_CORE, core)...)
	data = append(data, appendClientUserDataBlockForTest(nil, gccUserDataCS_MONITOR, monitor)...)

	settings := parseClientDisplaySettingsFromConnectInitial(data)
	if !settings.CoreDesktopPresent || settings.DesktopWidth != 1920 || settings.DesktopHeight != 1080 {
		t.Fatalf("unexpected core display settings: %#v", settings)
	}
	if !settings.MonitorLayoutPresent || settings.MonitorCount != 2 {
		t.Fatalf("unexpected monitor settings: %#v", settings)
	}
}

func TestParseClientDisplaySettingsFromConnectInitialMissingBlocks(t *testing.T) {
	settings := parseClientDisplaySettingsFromConnectInitial([]byte{0x01, 0x02, 0x03})
	if settings.CoreDesktopPresent || settings.MonitorLayoutPresent || settings.DesktopWidth != 0 || settings.DesktopHeight != 0 || settings.MonitorCount != 0 {
		t.Fatalf("unexpected settings for missing blocks: %#v", settings)
	}
}

func TestServerNetworkDataIncludesStaticChannelIDs(t *testing.T) {
	data := buildServerUserData(protocolRDP, []clientChannel{{Name: "rdpdr", ID: 1004}, {Name: "cliprdr", ID: 1005}})
	blocks := parseGCCUserDataBlocksForTest(t, data)
	network := blocks[gccUserDataSC_NET]
	if len(network) != 8 {
		t.Fatalf("network length = %d, want 8", len(network))
	}
	if got := binary.LittleEndian.Uint16(network[2:4]); got != 2 {
		t.Fatalf("channel count = %d, want 2", got)
	}
	if got := binary.LittleEndian.Uint16(network[4:6]); got != 1004 {
		t.Fatalf("first channel id = %d, want 1004", got)
	}
	if got := binary.LittleEndian.Uint16(network[6:8]); got != 1005 {
		t.Fatalf("second channel id = %d, want 1005", got)
	}
}

func appendClientUserDataBlockForTest(dst []byte, blockType uint16, payload []byte) []byte {
	dst = append(dst, byte(blockType), byte(blockType>>8))
	blockLen := uint16(len(payload) + 4)
	dst = append(dst, byte(blockLen), byte(blockLen>>8))
	dst = append(dst, payload...)
	return dst
}
