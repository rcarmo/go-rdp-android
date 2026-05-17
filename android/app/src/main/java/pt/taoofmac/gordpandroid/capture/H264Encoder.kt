package io.carmo.go.rdp.android.capture

import android.media.MediaCodec
import android.media.MediaCodecInfo
import android.media.MediaFormat
import android.os.Bundle
import android.os.Handler
import android.os.HandlerThread
import android.util.Log
import android.view.Surface
import java.nio.ByteBuffer

/**
 * Low-latency Android AVC/H.264 encoder scaffold for future RDPGFX H.264 transport.
 *
 * The encoder owns an input [Surface] suitable for a MediaProjection VirtualDisplay.
 * Encoded access units are returned through [Listener] with bounded, copy-out buffers
 * so the RDP bridge can decide whether to send them, drop stale frames, or fall back
 * to RDPGFX Planar/bitmap paths.
 */
class H264Encoder(
    private val listener: Listener,
) {
    interface Listener {
        fun onEncodedFrame(data: ByteArray, presentationTimeUs: Long, keyFrame: Boolean, codecConfig: Boolean)
        fun onEncoderError(error: Throwable)
    }

    data class Config(
        val width: Int,
        val height: Int,
        val bitRate: Int = 4_000_000,
        val frameRate: Int = 30,
        val keyFrameIntervalSeconds: Int = 1,
    ) {
        init {
            require(width > 0) { "width must be positive" }
            require(height > 0) { "height must be positive" }
            require(bitRate > 0) { "bitRate must be positive" }
            require(frameRate > 0) { "frameRate must be positive" }
            require(keyFrameIntervalSeconds >= 0) { "keyFrameIntervalSeconds must be non-negative" }
        }
    }

    private var codec: MediaCodec? = null
    private var inputSurface: Surface? = null
    private var thread: HandlerThread? = null
    private var started = false

    fun start(config: Config): Surface {
        stop()

        val encoderThread = HandlerThread("rdp-h264-encoder").also { it.start() }
        val encoderHandler = Handler(encoderThread.looper)
        val encoder = MediaCodec.createEncoderByType(MediaFormat.MIMETYPE_VIDEO_AVC)

        encoder.setCallback(object : MediaCodec.Callback() {
            override fun onInputBufferAvailable(codec: MediaCodec, index: Int) {
                // Surface input mode does not use raw input buffers.
            }

            override fun onOutputBufferAvailable(codec: MediaCodec, index: Int, info: MediaCodec.BufferInfo) {
                drainOutput(codec, index, info)
            }

            override fun onError(codec: MediaCodec, e: MediaCodec.CodecException) {
                listener.onEncoderError(e)
            }

            override fun onOutputFormatChanged(codec: MediaCodec, format: MediaFormat) {
                Log.i(TAG, "H.264 encoder output format changed: $format")
                codecConfigFromFormat(format)?.let { csd ->
                    listener.onEncodedFrame(csd, 0L, keyFrame = false, codecConfig = true)
                }
            }
        }, encoderHandler)

        val format = MediaFormat.createVideoFormat(MediaFormat.MIMETYPE_VIDEO_AVC, config.width, config.height).apply {
            setInteger(MediaFormat.KEY_COLOR_FORMAT, MediaCodecInfo.CodecCapabilities.COLOR_FormatSurface)
            setInteger(MediaFormat.KEY_BIT_RATE, config.bitRate)
            setInteger(MediaFormat.KEY_FRAME_RATE, config.frameRate)
            setInteger(MediaFormat.KEY_I_FRAME_INTERVAL, config.keyFrameIntervalSeconds)
            setInteger(MediaFormat.KEY_LATENCY, 0)
        }

        encoder.configure(format, null, null, MediaCodec.CONFIGURE_FLAG_ENCODE)
        val surface = encoder.createInputSurface()
        encoder.start()

        codec = encoder
        inputSurface = surface
        thread = encoderThread
        started = true
        Log.i(TAG, "H.264 encoder started ${config.width}x${config.height} bitrate=${config.bitRate} fps=${config.frameRate} keyInterval=${config.keyFrameIntervalSeconds}s")
        return surface
    }

    fun requestKeyFrame() {
        val encoder = codec ?: return
        runCatching {
            encoder.setParameters(Bundle().apply { putInt(MediaCodec.PARAMETER_KEY_REQUEST_SYNC_FRAME, 0) })
        }.onFailure { listener.onEncoderError(it) }
    }

    fun stop() {
        val encoder = codec
        codec = null
        inputSurface?.release()
        inputSurface = null
        if (encoder != null) {
            runCatching {
                if (started) encoder.stop()
            }.onFailure { Log.w(TAG, "H.264 encoder stop failed", it) }
            encoder.release()
        }
        started = false
        thread?.quitSafely()
        thread = null
    }

    private fun codecConfigFromFormat(format: MediaFormat): ByteArray? {
        val parts = listOf("csd-0", "csd-1", "csd-2").mapNotNull { key ->
            if (!format.containsKey(key)) return@mapNotNull null
            runCatching {
                val buffer = format.getByteBuffer(key) ?: return@mapNotNull null
                val dup = buffer.duplicate()
                val bytes = ByteArray(dup.remaining())
                dup.get(bytes)
                bytes.takeIf { it.isNotEmpty() }
            }.getOrNull()
        }
        if (parts.isEmpty()) return null
        val total = parts.sumOf { it.size }
        val out = ByteArray(total)
        var offset = 0
        for (part in parts) {
            part.copyInto(out, offset)
            offset += part.size
        }
        return out
    }

    private fun drainOutput(codec: MediaCodec, index: Int, info: MediaCodec.BufferInfo) {
        if (info.size <= 0) {
            codec.releaseOutputBuffer(index, false)
            return
        }
        val buffer: ByteBuffer? = codec.getOutputBuffer(index)
        if (buffer == null) {
            codec.releaseOutputBuffer(index, false)
            return
        }
        buffer.position(info.offset)
        buffer.limit(info.offset + info.size)
        val data = ByteArray(info.size)
        buffer.get(data)
        val flags = info.flags
        val keyFrame = flags and MediaCodec.BUFFER_FLAG_KEY_FRAME != 0
        val codecConfig = flags and MediaCodec.BUFFER_FLAG_CODEC_CONFIG != 0
        codec.releaseOutputBuffer(index, false)
        listener.onEncodedFrame(data, info.presentationTimeUs, keyFrame, codecConfig)
    }

    companion object {
        private const val TAG = "GoRdpAndroidH264"
    }
}
