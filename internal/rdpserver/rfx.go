package rdpserver

const rfxMaxEncodedPayloadLen = rdpgfxMaxPDUSize

func buildRFXSurfaceBitsCommand(width, height int, codecID byte, encoded []byte) ([]byte, bool) {
	if len(encoded) == 0 || len(encoded) > rfxMaxEncodedPayloadLen {
		return nil, false
	}
	return buildSurfaceBitsCommand(width, height, codecID, encoded)
}
