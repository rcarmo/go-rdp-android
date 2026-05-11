package io.carmo.go.rdp.android.bridge

import android.util.Log
import io.carmo.go.rdp.android.input.RdpAccessibilityService
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong

/**
 * Android-facing bridge. It prefers the gomobile-generated Go backend when
 * `mobile.aar` is present under `android/app/libs/`, and falls back to a
 * logging implementation so the app remains buildable in CI before the AAR is generated.
 */
object NativeRdpBridge : RdpInputCallbacks {
    private val frameCount = AtomicLong(0)
    private val running = AtomicBoolean(false)
    @Volatile private var lastMode: String = "stopped"
    private val inputCoordinateScale = AtomicInteger(1)
    private val backend: RdpBackend by lazy {
        val go = GomobileRdpBackend()
        if (go.available) go else LoggingRdpBackend()
    }

    fun setInputCoordinateScale(scale: Int) {
        val normalized = scale.coerceIn(1, 4)
        inputCoordinateScale.set(normalized)
        Log.i(TAG, "inputCoordinateScale=$normalized")
    }

    fun setCredentials(username: String, password: String) {
        backend.setCredentials(username, password)
    }

    fun startServer(port: Int, hasProjection: Boolean) {
        backend.setInputCallbacks(this)
        backend.startServer(port)
        frameCount.set(0)
        running.set(true)
        lastMode = if (hasProjection) "screen capture" else "test pattern / no projection"
        Log.i(TAG, "startServer(port=$port, hasProjection=$hasProjection, backend=${backend.name})")
    }

    fun submitFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray) {
        val count = frameCount.incrementAndGet()
        if (count == 1L || count % 120L == 0L) {
            Log.i(TAG, "frame#$count ${width}x$height pixelStride=$pixelStride rowStride=$rowStride bytes=${data.size} backend=${backend.name}")
        }
        backend.submitFrame(width, height, pixelStride, rowStride, data)
    }

    override fun onPointerMove(x: Int, y: Int) {
        val scale = inputCoordinateScale.get()
        RdpAccessibilityService.handlePointerMove(x * scale, y * scale)
    }

    override fun onPointerButton(x: Int, y: Int, buttons: Int, down: Boolean) {
        val scale = inputCoordinateScale.get()
        RdpAccessibilityService.handlePointerButton(x * scale, y * scale, buttons, down)
    }

    override fun onPointerWheel(x: Int, y: Int, delta: Int, horizontal: Boolean) {
        val scale = inputCoordinateScale.get()
        RdpAccessibilityService.handlePointerWheel(x * scale, y * scale, delta, horizontal)
    }

    override fun onKey(scancode: Int, down: Boolean) {
        RdpAccessibilityService.handleKey(scancode, down)
    }

    override fun onUnicode(codepoint: Int) {
        RdpAccessibilityService.handleUnicode(codepoint)
    }

    override fun onTouchFrameStart(contactCount: Int) {
        RdpAccessibilityService.handleTouchFrameStart(contactCount)
    }

    override fun onTouchContact(contactId: Int, x: Int, y: Int, flags: Int) {
        val scale = inputCoordinateScale.get()
        RdpAccessibilityService.handleTouchContact(contactId, x * scale, y * scale, flags)
    }

    override fun onTouchFrameEnd() {
        RdpAccessibilityService.handleTouchFrameEnd()
    }

    fun stopServer() {
        backend.stopServer()
        running.set(false)
        lastMode = "stopped"
        Log.i(TAG, "stopServer(backend=${backend.name})")
    }

    fun healthStatus(): String = "backend=${backend.name}, running=${running.get()}, mode=$lastMode, frames=${frameCount.get()}, inputScale=${inputCoordinateScale.get()}"

    private const val TAG = "GoRdpAndroid"
}
