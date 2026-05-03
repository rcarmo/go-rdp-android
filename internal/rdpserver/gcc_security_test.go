package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestBuildServerUserDataIncludesCoreSecurityAndNetwork(t *testing.T) {
	data := buildServerUserData(protocolSSL, nil)
	blocks := parseGCCUserDataBlocksForTest(t, data)

	core := blocks[gccUserDataSC_CORE]
	if len(core) != 12 {
		t.Fatalf("core payload length = %d, want 12", len(core))
	}
	if got := binary.LittleEndian.Uint32(core[0:4]); got != rdpVersion10 {
		t.Fatalf("server core version = 0x%08x, want 0x%08x", got, rdpVersion10)
	}
	if got := binary.LittleEndian.Uint32(core[4:8]); got != protocolSSL {
		t.Fatalf("selected protocol = 0x%08x, want SSL", got)
	}

	security := blocks[gccUserDataSC_SECURITY]
	if len(security) != 8 {
		t.Fatalf("security payload length = %d, want 8", len(security))
	}
	if method := binary.LittleEndian.Uint32(security[0:4]); method != 0 {
		t.Fatalf("encryption method = %d, want none", method)
	}
	if level := binary.LittleEndian.Uint32(security[4:8]); level != 0 {
		t.Fatalf("encryption level = %d, want none", level)
	}

	network := blocks[gccUserDataSC_NET]
	if len(network) != 4 {
		t.Fatalf("network payload length = %d, want 4", len(network))
	}
	if channel := binary.LittleEndian.Uint16(network[0:2]); channel != globalChannelID {
		t.Fatalf("network MCS channel = %d, want %d", channel, globalChannelID)
	}
	if count := binary.LittleEndian.Uint16(network[2:4]); count != 0 {
		t.Fatalf("network channel count = %d, want 0", count)
	}
}

func parseGCCUserDataBlocksForTest(t *testing.T, data []byte) map[uint16][]byte {
	t.Helper()
	blocks := map[uint16][]byte{}
	for off := 0; off < len(data); {
		if off+4 > len(data) {
			t.Fatalf("truncated GCC user data header at %d", off)
		}
		blockType := binary.LittleEndian.Uint16(data[off : off+2])
		blockLen := int(binary.LittleEndian.Uint16(data[off+2 : off+4]))
		if blockLen < 4 || off+blockLen > len(data) {
			t.Fatalf("invalid block 0x%04x length %d at %d", blockType, blockLen, off)
		}
		blocks[blockType] = data[off+4 : off+blockLen]
		off += blockLen
	}
	return blocks
}
