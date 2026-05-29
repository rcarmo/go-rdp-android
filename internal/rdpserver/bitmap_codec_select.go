package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

type bitmapCodecCommand struct {
	Name     string
	CodecID  byte
	Command  []byte
	Quality  int
	Trace    string
	RawBytes int
}

func buildExperimentalBitmapCodecCommand(src frame.Frame, caps confirmActiveCapabilities) (bitmapCodecCommand, bool) {
	rawBytes := src.Width * src.Height * 4
	if codecID, ok := negotiatedNSCodecID(caps); ok {
		if command, built := buildNSCodecSurfaceBitsCommand(src, codecID); built {
			return bitmapCodecCommand{Name: "nscodec", CodecID: codecID, Command: command, Trace: "nscodec", RawBytes: rawBytes}, true
		}
	}
	if codecID, ok := negotiatedJPEGCodecID(caps); ok {
		quality := jpegQualityFromEnv()
		if command, built := buildJPEGSurfaceBitsCommand(src, codecID, quality); built {
			return bitmapCodecCommand{Name: "jpeg-codec", CodecID: codecID, Command: command, Quality: quality, Trace: "jpeg_codec", RawBytes: rawBytes}, true
		}
	}
	if codecID, ok := negotiatedPNGCodecID(); ok {
		if command, built := buildPNGSurfaceBitsCommand(src, codecID); built {
			return bitmapCodecCommand{Name: "png-codec", CodecID: codecID, Command: command, Trace: "png_codec", RawBytes: rawBytes}, true
		}
	}
	return bitmapCodecCommand{}, false
}

func buildRemoteFXBitmapCodecCommand(src frame.Frame, caps confirmActiveCapabilities, encoder RFXEncoder) (bitmapCodecCommand, bool) {
	codecID, ok := negotiatedRemoteFXCodecID(caps)
	if !ok {
		return bitmapCodecCommand{}, false
	}
	if encoder == nil {
		tracef("rfx_codec_selected", "codec_id=%d emission=deferred reason=encoder-missing", codecID)
		return bitmapCodecCommand{}, false
	}
	encoded, encodedOK := encoder.EncodeRFX(src, src.Width, src.Height)
	if !encodedOK {
		tracef("rfx_codec_selected", "codec_id=%d emission=deferred reason=encoder-rejected", codecID)
		return bitmapCodecCommand{}, false
	}
	command, built := buildRFXSurfaceBitsCommand(src.Width, src.Height, codecID, encoded)
	if !built {
		tracef("rfx_codec_selected", "codec_id=%d emission=deferred reason=surfacebits-build-failed", codecID)
		return bitmapCodecCommand{}, false
	}
	return bitmapCodecCommand{Name: "rfx-codec", CodecID: codecID, Command: command, Trace: "rfx_codec", RawBytes: src.Width * src.Height * 4}, true
}

func recordExperimentalBitmapCodecFrame(metrics serverMetrics, cmd bitmapCodecCommand) bool {
	switch cmd.Name {
	case "nscodec":
		metrics.recordNSCodecFrameSavings([][]byte{cmd.Command}, int64(cmd.RawBytes), int64(cmd.savedBytes()))
		return true
	case "jpeg-codec":
		metrics.recordJPEGCodecFrameSavings([][]byte{cmd.Command}, int64(cmd.RawBytes), int64(cmd.savedBytes()))
		return true
	case "png-codec":
		metrics.recordPNGCodecFrameSavings([][]byte{cmd.Command}, int64(cmd.RawBytes), int64(cmd.savedBytes()))
		return true
	case "rfx-codec":
		metrics.recordRFXCodecFrame([][]byte{cmd.Command}, int64(cmd.RawBytes), int64(cmd.savedBytes()))
		return true
	default:
		return false
	}
}

func (cmd bitmapCodecCommand) savedBytes() int {
	if cmd.RawBytes <= len(cmd.Command) {
		return 0
	}
	return cmd.RawBytes - len(cmd.Command)
}

func traceExperimentalBitmapCodecSelected(cmd bitmapCodecCommand) {
	switch cmd.Trace {
	case "nscodec":
		tracef("nscodec_selected", "codec_id=%d command_bytes=%d raw_bytes=%d saved_bytes=%d emission=opt-in", cmd.CodecID, len(cmd.Command), cmd.RawBytes, cmd.savedBytes())
	case "jpeg_codec":
		tracef("jpeg_codec_selected", "codec_id=%d command_bytes=%d raw_bytes=%d saved_bytes=%d quality=%d emission=opt-in", cmd.CodecID, len(cmd.Command), cmd.RawBytes, cmd.savedBytes(), cmd.Quality)
	case "png_codec":
		tracef("png_codec_selected", "codec_id=%d command_bytes=%d raw_bytes=%d saved_bytes=%d emission=operator-override", cmd.CodecID, len(cmd.Command), cmd.RawBytes, cmd.savedBytes())
	case "rfx_codec":
		tracef("rfx_codec_selected", "codec_id=%d command_bytes=%d raw_bytes=%d saved_bytes=%d emission=opt-in", cmd.CodecID, len(cmd.Command), cmd.RawBytes, cmd.savedBytes())
	}
}

func traceExperimentalBitmapCodecWrite(cmd bitmapCodecCommand) {
	switch cmd.Trace {
	case "nscodec":
		tracef("nscodec_write", "codec_id=%d bytes=%d raw_bytes=%d saved_bytes=%d", cmd.CodecID, len(cmd.Command), cmd.RawBytes, cmd.savedBytes())
	case "jpeg_codec":
		tracef("jpeg_codec_write", "codec_id=%d bytes=%d raw_bytes=%d saved_bytes=%d", cmd.CodecID, len(cmd.Command), cmd.RawBytes, cmd.savedBytes())
	case "png_codec":
		tracef("png_codec_write", "codec_id=%d bytes=%d raw_bytes=%d saved_bytes=%d", cmd.CodecID, len(cmd.Command), cmd.RawBytes, cmd.savedBytes())
	case "rfx_codec":
		tracef("rfx_codec_write", "codec_id=%d bytes=%d raw_bytes=%d saved_bytes=%d", cmd.CodecID, len(cmd.Command), cmd.RawBytes, cmd.savedBytes())
	}
}
