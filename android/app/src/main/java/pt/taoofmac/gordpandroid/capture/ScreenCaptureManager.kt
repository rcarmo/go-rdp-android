package pt.taoofmac.gordpandroid.capture

import android.content.Context
import android.content.Intent
import android.graphics.ImageFormat
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
 * Frames are throttled before copying from ImageReader so the future Go bridge
 * does not get overwhelmed by full-rate display updates on high-refresh devices.
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
    private var minFrameIntervalMs: Long = 66 // ~15 FPS initially
    private var lastFrameAtMs: Long = 0
    private var stopping = false

    fun start(resultCode: Int, data: Intent, width: Int, height: Int, densityDpi: Int, maxFps: Int = 15) {
        stop()
        stopping = false
        minFrameIntervalMs = if (maxFps <= 0) 0 else (1000L / maxFps.coerceAtLeast(1))
        lastFrameAtMs = 0

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

        imageReader = ImageReader.newInstance(width, height, ImageFormat.RGBA_8888, 2).also { reader ->
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
        Log.i("GoRdpAndroid", "Screen capture started ${width}x$height density=$densityDpi maxFps=$maxFps")
    }

    fun stop() {
        stopping = true
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
            if (minFrameIntervalMs > 0 && now - lastFrameAtMs < minFrameIntervalMs) {
                return
            }
            lastFrameAtMs = now

            val plane: Image.Plane = img.planes.firstOrNull() ?: return
            val buffer = plane.buffer
            val data = ByteArray(buffer.remaining())
            buffer.get(data)
            listener.onFrame(img.width, img.height, plane.pixelStride, plane.rowStride, data)
        }
    }
}
