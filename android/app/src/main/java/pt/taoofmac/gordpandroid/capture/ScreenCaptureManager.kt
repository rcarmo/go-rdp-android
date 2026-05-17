package io.carmo.go.rdp.android.capture

import android.content.Context
import android.content.Intent
import android.graphics.PixelFormat
import android.hardware.display.DisplayManager
import android.hardware.display.VirtualDisplay
import android.media.Image
import android.media.ImageReader
import android.media.projection.MediaProjection
import android.media.projection.MediaProjectionManager
import android.os.Handler
import android.os.HandlerThread
import android.os.SystemClock
import android.util.Log

/**
 * MediaProjection-backed frame source.
 *
 * Frames are throttled before copying from ImageReader so the Go bridge does
 * not get overwhelmed by full-rate display updates on high-refresh devices.
 * The throttle is adaptive: if bridge submission takes longer than the target
 * interval, capture slows down; if submission is cheap, it gradually returns to
 * the configured target FPS.
 */
class ScreenCaptureManager(
    private val context: Context,
    private val listener: Listener,
) {
    interface Listener {
        fun onFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray)
        fun onEncodedFrame(data: ByteArray, presentationTimeUs: Long, keyFrame: Boolean, codecConfig: Boolean) {}
        fun onEncoderError(error: Throwable) {}
        fun onStopped()
    }

    private var projection: MediaProjection? = null
    private var imageReader: ImageReader? = null
    private var h264Encoder: H264Encoder? = null
    private var virtualDisplay: VirtualDisplay? = null
    private var thread: HandlerThread? = null
    private var handler: Handler? = null
    private var targetFrameIntervalMs: Long = 66 // ~15 FPS initially
    private var adaptiveFrameIntervalMs: Long = 66
    private var lastFrameCompletedAtMs: Long = 0
    private var stopping = false
    private var submittedFrames: Long = 0
    private var throttledFrames: Long = 0
    private var copiedBytes: Long = 0
    private var totalSubmitMs: Long = 0
    private var maxSubmitMs: Long = 0
    private var lastStatsLogMs: Long = 0

    fun start(resultCode: Int, data: Intent, width: Int, height: Int, densityDpi: Int, maxFps: Int = 15) {
        stop()
        stopping = false
        targetFrameIntervalMs = if (maxFps <= 0) 0 else (1000L / maxFps.coerceAtLeast(1))
        adaptiveFrameIntervalMs = targetFrameIntervalMs
        resetStats()

        val manager = context.getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        val mediaProjection = manager.getMediaProjection(resultCode, data).also { mp ->
            mp.registerCallback(object : MediaProjection.Callback() {
                override fun onStop() {
                    if (!stopping) {
                        stop()
                        listener.onStopped()
                    }
                }
            }, null)
        }
        projection = mediaProjection

        val captureThread = HandlerThread("rdp-capture").also { it.start() }
        val captureHandler = Handler(captureThread.looper)
        thread = captureThread
        handler = captureHandler

        val reader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 2).also { imageReader ->
            imageReader.setOnImageAvailableListener({ onImageAvailable(it) }, captureHandler)
        }
        imageReader = reader

        virtualDisplay = mediaProjection.createVirtualDisplay(
            "go-rdp-android",
            width,
            height,
            densityDpi,
            DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            reader.surface,
            null,
            captureHandler,
        )
        Log.i(TAG, "Screen capture started ${width}x$height density=$densityDpi maxFps=$maxFps targetIntervalMs=$targetFrameIntervalMs")
    }

    fun startH264(resultCode: Int, data: Intent, width: Int, height: Int, densityDpi: Int, config: H264Encoder.Config = H264Encoder.Config(width, height)) {
        stop()
        stopping = false
        targetFrameIntervalMs = 1000L / config.frameRate.coerceAtLeast(1)
        adaptiveFrameIntervalMs = targetFrameIntervalMs
        resetStats()

        val manager = context.getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        val mediaProjection = manager.getMediaProjection(resultCode, data).also { mp ->
            mp.registerCallback(object : MediaProjection.Callback() {
                override fun onStop() {
                    if (!stopping) {
                        stop()
                        listener.onStopped()
                    }
                }
            }, null)
        }
        projection = mediaProjection

        val captureThread = HandlerThread("rdp-capture-h264").also { it.start() }
        val captureHandler = Handler(captureThread.looper)
        thread = captureThread
        handler = captureHandler

        val encoder = H264Encoder(object : H264Encoder.Listener {
            override fun onEncodedFrame(data: ByteArray, presentationTimeUs: Long, keyFrame: Boolean, codecConfig: Boolean) {
                submittedFrames += 1
                copiedBytes += data.size.toLong()
                if (keyFrame) Log.i(TAG, "H.264 keyframe bytes=${data.size} pts=$presentationTimeUs")
                logStats(force = false)
                listener.onEncodedFrame(data, presentationTimeUs, keyFrame, codecConfig)
            }

            override fun onEncoderError(error: Throwable) {
                Log.w(TAG, "H.264 encoder error", error)
                listener.onEncoderError(error)
            }
        })
        h264Encoder = encoder
        val surface = encoder.start(config)

        virtualDisplay = mediaProjection.createVirtualDisplay(
            "go-rdp-android-h264",
            width,
            height,
            densityDpi,
            DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            surface,
            null,
            captureHandler,
        )
        Log.i(TAG, "H.264 screen capture started ${width}x$height density=$densityDpi bitrate=${config.bitRate} fps=${config.frameRate}")
    }

    fun stop() {
        stopping = true
        logStats(force = true)
        virtualDisplay?.release()
        virtualDisplay = null
        h264Encoder?.stop()
        h264Encoder = null
        imageReader?.close()
        imageReader = null
        val oldProjection = projection
        projection = null
        oldProjection?.stop()
        thread?.quitSafely()
        thread = null
        handler = null
    }

    private fun resetStats() {
        lastFrameCompletedAtMs = 0
        submittedFrames = 0
        throttledFrames = 0
        copiedBytes = 0
        totalSubmitMs = 0
        maxSubmitMs = 0
        lastStatsLogMs = SystemClock.elapsedRealtime()
    }

    private fun onImageAvailable(reader: ImageReader) {
        val image = reader.acquireLatestImage() ?: return
        image.use { img ->
            val now = SystemClock.elapsedRealtime()
            if (adaptiveFrameIntervalMs > 0 && lastFrameCompletedAtMs > 0 && now - lastFrameCompletedAtMs < adaptiveFrameIntervalMs) {
                throttledFrames += 1
                logStats(force = false)
                return
            }

            val plane: Image.Plane = img.planes.firstOrNull() ?: return
            val buffer = plane.buffer
            val data = ByteArray(buffer.remaining())
            buffer.get(data)
            copiedBytes += data.size.toLong()

            val submitStarted = SystemClock.elapsedRealtime()
            listener.onFrame(img.width, img.height, plane.pixelStride, plane.rowStride, data)
            val submitMs = SystemClock.elapsedRealtime() - submitStarted
            submittedFrames += 1
            totalSubmitMs += submitMs
            if (submitMs > maxSubmitMs) maxSubmitMs = submitMs
            adjustBackpressure(submitMs)
            lastFrameCompletedAtMs = SystemClock.elapsedRealtime()
            logStats(force = false)
        }
    }

    private fun adjustBackpressure(submitMs: Long) {
        if (targetFrameIntervalMs <= 0) return
        val maxInterval = (targetFrameIntervalMs * 4).coerceAtLeast(targetFrameIntervalMs)
        adaptiveFrameIntervalMs = when {
            submitMs > adaptiveFrameIntervalMs -> (submitMs * 2).coerceAtMost(maxInterval)
            adaptiveFrameIntervalMs > targetFrameIntervalMs && submitMs * 3 < adaptiveFrameIntervalMs ->
                (adaptiveFrameIntervalMs - targetFrameIntervalMs / 2).coerceAtLeast(targetFrameIntervalMs)
            else -> adaptiveFrameIntervalMs
        }
    }

    private fun logStats(force: Boolean) {
        val now = SystemClock.elapsedRealtime()
        if (!force && submittedFrames != 1L && submittedFrames % 30L != 0L && now - lastStatsLogMs < 5_000L) return
        if (submittedFrames == 0L && throttledFrames == 0L) return
        lastStatsLogMs = now
        val avgSubmitMs = if (submittedFrames == 0L) 0 else totalSubmitMs / submittedFrames
        Log.i(
            TAG,
            "captureStats submitted=$submittedFrames throttled=$throttledFrames copiedBytes=$copiedBytes " +
                "avgSubmitMs=$avgSubmitMs maxSubmitMs=$maxSubmitMs adaptiveIntervalMs=$adaptiveFrameIntervalMs targetIntervalMs=$targetFrameIntervalMs",
        )
    }

    companion object {
        private const val TAG = "GoRdpAndroidCapture"
    }
}
