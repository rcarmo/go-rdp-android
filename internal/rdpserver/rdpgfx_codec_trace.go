package rdpserver

func traceDeferredRDPGFXCodecSelection(cap rdpgfxCapabilitySet) {
	if rdpgfxCapabilitySupportsClearCodec(cap) {
		tracef("rdpgfx_clearcodec_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x emission=deferred reason=encoder-missing", cap.Version, cap.Flags, rdpgfxCodecClearCodec)
	}
	if rdpgfxCapabilitySupportsProgressive(cap) {
		tracef("rdpgfx_progressive_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x codec_id_v2=0x%04x emission=deferred reason=encoder-missing", cap.Version, cap.Flags, rdpgfxCodecCAProgressive, rdpgfxCodecCAProgressiveV2)
	}
	if rdpgfxCapabilitySupportsAVC444(cap) {
		tracef("rdpgfx_avc444_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x emission=deferred reason=transport-missing", cap.Version, cap.Flags, rdpgfxCodecAVC444)
	}
	if rdpgfxCapabilitySupportsAVC444v2(cap) {
		tracef("rdpgfx_avc444v2_selected", "version=0x%08x flags=0x%08x codec_id=0x%04x emission=deferred reason=transport-missing", cap.Version, cap.Flags, rdpgfxCodecAVC444v2)
	}
}
