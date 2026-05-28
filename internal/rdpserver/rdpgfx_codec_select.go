package rdpserver

import (
	"os"
	"strings"
)

func rdpgfxCodecEnvEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func clearCodecEnabledFromEnv() bool {
	return rdpgfxCodecEnvEnabled("GO_RDP_ANDROID_ENABLE_CLEARCODEC")
}
func progressiveCodecEnabledFromEnv() bool {
	return rdpgfxCodecEnvEnabled("GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC")
}
func avc444EnabledFromEnv() bool   { return rdpgfxCodecEnvEnabled("GO_RDP_ANDROID_ENABLE_AVC444") }
func avc444v2EnabledFromEnv() bool { return rdpgfxCodecEnvEnabled("GO_RDP_ANDROID_ENABLE_AVC444V2") }

func rdpgfxCapabilitySupportsClearCodec(cap rdpgfxCapabilitySet) bool {
	return clearCodecEnabledFromEnv() && cap.Version >= rdpgfxCapsVersion8
}

func rdpgfxCapabilitySupportsProgressive(cap rdpgfxCapabilitySet) bool {
	return progressiveCodecEnabledFromEnv() && cap.Version >= rdpgfxCapsVersion10
}

func rdpgfxCapabilitySupportsProgressiveV2(cap rdpgfxCapabilitySet) bool {
	return progressiveCodecEnabledFromEnv() && cap.Version >= rdpgfxCapsVersion104
}

func rdpgfxCapabilitySupportsAVC444(cap rdpgfxCapabilitySet) bool {
	return avc444EnabledFromEnv() && cap.Version >= rdpgfxCapsVersion10 && cap.Flags&rdpgfxCapsFlagAVCDisabled == 0
}

func rdpgfxCapabilitySupportsAVC444v2(cap rdpgfxCapabilitySet) bool {
	return avc444v2EnabledFromEnv() && cap.Version >= rdpgfxCapsVersion104 && cap.Flags&rdpgfxCapsFlagAVCDisabled == 0
}
