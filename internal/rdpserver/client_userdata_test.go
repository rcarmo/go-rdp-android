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
