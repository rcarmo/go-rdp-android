package pt.taoofmac.gordpandroid.capture

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
        fun onStopped()
    }

    private var projection: MediaProjection? = null
    private var imageReader: ImageReader? = null
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
        lastFrameCompletedAtMs = 0
        submittedFrames = 0
        throttledFrames = 0
        copiedBytes = 0
        totalSubmitMs = 0
        maxSubmitMs = 0
        lastStatsLogMs = SystemClock.elapsedRealtime()

        val manager = context.getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        projection = manager.getMediaProjection(resultCode, data).also { mp ->
            mp.registerCallback(object : MediaProjection.Callback() {
                override fun onStop() {
                    if (!stopping) {
                        stop()
                        listener.onStopped()
                    }
                }
            }, null)
        }

        thread = HandlerThread("rdp-capture").also { it.start() }
        handler = Handler(thread!!.looper)

        imageReader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 2).also { reader ->
            reader.setOnImageAvailableListener({ onImageAvailable(it) }, handler)
        }

        virtualDisplay = projection!!.createVirtualDisplay(
            "go-rdp-android",
            width,
            height,
            densityDpi,
            DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            imageReader!!.surface,
            null,
            handler,
        )
        Log.i(TAG, "Screen capture started ${width}x$height density=$densityDpi maxFps=$maxFps targetIntervalMs=$targetFrameIntervalMs")
    }

    fun stop() {
        stopping = true
        logStats(force = true)
        virtualDisplay?.release()
        virtualDisplay = null
        imageReader?.close()
        imageReader = null
        val oldProjection = projection
        projection = null
        oldProjection?.stop()
        thread?.quitSafely()
        thread = null
        handler = null
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
