package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

type productionProgressiveV2Encoder struct{}

func (productionProgressiveV2Encoder) EncodeRDPGFX(src frame.Frame, width, height int) ([]byte, bool) {
	base, ok := (productionProgressiveEncoder{}).EncodeRDPGFX(src, width, height)
	if !ok {
		return nil, false
	}
	parsed, ok := parseProgressivePayload(base)
	if !ok {
		return nil, false
	}
	// Mark V2 payloads with the high quant bit while preserving the same bounded
	// single-layer region/data structure. This keeps V1 and V2 production paths
	// distinct at the wire-payload level until a fuller V2 progressive layer model
	// is implemented.
	parsed.Quant |= 0x80
	out, ok := buildProgressivePayload(parsed)
	if !ok || len(out) != len(base) {
		return nil, false
	}
	return out, true
}
