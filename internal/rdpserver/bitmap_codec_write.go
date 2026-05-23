package rdpserver

import (
	"net"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func writeExperimentalBitmapCodecUpdate(conn net.Conn, metrics serverMetrics, cmd bitmapCodecCommand) error {
	traceExperimentalBitmapCodecSelected(cmd)
	if err := writeShareDataPDU(conn, pduType2Update, cmd.Command); err != nil {
		return err
	}
	recordExperimentalBitmapCodecFrame(metrics, cmd)
	traceExperimentalBitmapCodecWrite(cmd)
	return nil
}

func streamExperimentalBitmapCodecUpdates(conn net.Conn, frames frame.Source, caps confirmActiveCapabilities, width, height int, metrics serverMetrics) {
	if frames == nil {
		return
	}
	frameCh := frames.Frames()
	for fr := range frameCh {
		fr = latestAvailableFrame(frameCh, fr)
		normalized := normalizeFrameForDesktop(fr, width, height)
		cmd, ok := buildExperimentalBitmapCodecCommand(normalized, caps)
		if !ok {
			continue
		}
		if err := writeExperimentalBitmapCodecUpdate(conn, metrics, cmd); err != nil {
			tracef("bitmap_codec_stream_stop", "path=%s err=%v", cmd.Name, err)
			return
		}
	}
}
