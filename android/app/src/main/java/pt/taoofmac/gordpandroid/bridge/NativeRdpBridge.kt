package pt.taoofmac.gordpandroid.bridge

import android.util.Log
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import pt.taoofmac.gordpandroid.input.RdpAccessibilityService

/**
 * Temporary Kotlin stub.
 *
 * This is the seam where a gomobile-generated Go binding should be wired in.
 * The methods and data shapes here intentionally mirror the planned Go API.
 */
object NativeRdpBridge {
    private val running = AtomicBoolean(false)
    private val frameCount = AtomicLong(0)

    fun startServer(port: Int, hasProjection: Boolean) {
        if (running.compareAndSet(false, true)) {
            frameCount.set(0)
            Log.i("GoRdpAndroid", "startServer(port=$port, hasProjection=$hasProjection) [stub]")
        } else {
            Log.i("GoRdpAndroid", "startServer ignored; already running [stub]")
        }
    }

    fun submitFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray) {
        if (!running.get()) return
        val count = frameCount.incrementAndGet()
        if (count == 1L || count % 120 == 0L) {
            Log.i("GoRdpAndroid", "frame#$count ${width}x$height pixelStride=$pixelStride rowStride=$rowStride bytes=${data.size} [stub]")
        }
    }

    fun onPointerMove(x: Int, y: Int) {
        RdpAccessibilityService.handlePointerMove(x, y)
    }

    fun onPointerButton(x: Int, y: Int, buttons: Int, down: Boolean) {
        RdpAccessibilityService.handlePointerButton(x, y, buttons, down)
    }

    fun onKey(scancode: Int, down: Boolean) {
        RdpAccessibilityService.handleKey(scancode, down)
    }

    fun onUnicode(codepoint: Int) {
        RdpAccessibilityService.handleUnicode(codepoint)
    }

    fun stopServer() {
        if (running.compareAndSet(true, false)) {
            Log.i("GoRdpAndroid", "stopServer(frames=${frameCount.get()}) [stub]")
        }
    }
}
