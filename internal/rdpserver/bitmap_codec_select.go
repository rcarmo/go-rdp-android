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

func recordExperimentalBitmapCodecFrame(metrics serverMetrics, cmd bitmapCodecCommand) bool {
	switch cmd.Name {
	case "nscodec":
		metrics.recordNSCodecFrame([][]byte{cmd.Command})
		return true
	case "jpeg-codec":
		metrics.recordJPEGCodecFrame([][]byte{cmd.Command})
		return true
	case "png-codec":
		metrics.recordPNGCodecFrame([][]byte{cmd.Command})
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
	}
}
