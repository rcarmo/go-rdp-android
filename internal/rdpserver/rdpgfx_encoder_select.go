package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

type rdpgfxEncodedPath struct {
	CodecID uint16
	Name    string
	Trace   string
	Encoder RDPGFXFrameEncoder
}

func selectRDPGFXEncodedPath(cap rdpgfxCapabilitySet, metrics serverMetrics) (rdpgfxEncodedPath, bool) {
	if rdpgfxCapabilitySupportsClearCodec(cap) && metrics.clearCodecEncoder != nil {
		return rdpgfxEncodedPath{CodecID: rdpgfxCodecClearCodec, Name: "rdpgfx-clearcodec", Trace: "rdpgfx_clearcodec", Encoder: metrics.clearCodecEncoder}, true
	}
	if rdpgfxCapabilitySupportsProgressiveV2(cap) && metrics.progressiveV2Encoder != nil {
		return rdpgfxEncodedPath{CodecID: rdpgfxCodecCAProgressiveV2, Name: "rdpgfx-progressive-v2", Trace: "rdpgfx_progressive_v2", Encoder: metrics.progressiveV2Encoder}, true
	}
	if rdpgfxCapabilitySupportsProgressive(cap) && metrics.progressiveEncoder != nil {
		return rdpgfxEncodedPath{CodecID: rdpgfxCodecCAProgressive, Name: "rdpgfx-progressive", Trace: "rdpgfx_progressive", Encoder: metrics.progressiveEncoder}, true
	}
	if rdpgfxCapabilitySupportsAVC444v2(cap) && metrics.avc444v2Encoder != nil {
		return rdpgfxEncodedPath{CodecID: rdpgfxCodecAVC444v2, Name: "rdpgfx-avc444v2", Trace: "rdpgfx_avc444v2", Encoder: metrics.avc444v2Encoder}, true
	}
	if rdpgfxCapabilitySupportsAVC444(cap) && metrics.avc444Encoder != nil {
		return rdpgfxEncodedPath{CodecID: rdpgfxCodecAVC444, Name: "rdpgfx-avc444", Trace: "rdpgfx_avc444", Encoder: metrics.avc444Encoder}, true
	}
	return rdpgfxEncodedPath{}, false
}

func buildSelectedRDPGFXEncodedFramePDUs(surfaceID uint16, frameID uint32, fr frame.Frame, width, height int, cap rdpgfxCapabilitySet, metrics serverMetrics) ([][]byte, string, bool) {
	selected, ok := selectRDPGFXEncodedPath(cap, metrics)
	if !ok {
		return nil, "", false
	}
	pdus, path, ok := buildRDPGFXEncodedFramePDUs(surfaceID, frameID, fr, width, height, selected.CodecID, selected.Name, selected.Encoder)
	if ok {
		traceSelectedRDPGFXEncodedPath(cap, selected, "opt-in")
		return pdus, path, true
	}
	traceSelectedRDPGFXEncodedPath(cap, selected, "deferred reason=encoder-rejected-frame")
	return nil, "", false
}

func traceSelectedRDPGFXEncodedPath(cap rdpgfxCapabilitySet, selected rdpgfxEncodedPath, emission string) {
	switch selected.Trace {
	case "rdpgfx_clearcodec":
		tracef("rdpgfx_clearcodec_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x emission=%s", cap.Version, cap.Flags, rdpgfxCodecClearCodec, emission)
	case "rdpgfx_progressive":
		tracef("rdpgfx_progressive_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x codec_id_v2=0x%04x emission=%s", cap.Version, cap.Flags, rdpgfxCodecCAProgressive, rdpgfxCodecCAProgressiveV2, emission)
	case "rdpgfx_progressive_v2":
		tracef("rdpgfx_progressive_v2_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x emission=%s", cap.Version, cap.Flags, rdpgfxCodecCAProgressiveV2, emission)
	case "rdpgfx_avc444":
		tracef("rdpgfx_avc444_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x emission=%s", cap.Version, cap.Flags, rdpgfxCodecAVC444, emission)
	case "rdpgfx_avc444v2":
		tracef("rdpgfx_avc444v2_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x emission=%s", cap.Version, cap.Flags, rdpgfxCodecAVC444v2, emission)
	}
}
