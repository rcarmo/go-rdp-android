package io.carmo.go.rdp.android.bridge

import android.util.Log
import io.carmo.go.rdp.android.input.RdpAccessibilityService

/**
 * Android-facing bridge. It prefers the gomobile-generated Go backend when
 * `mobile.aar` is present under `android/app/libs/`, and falls back to a
 * logging implementation so the app remains buildable in CI before the AAR is generated.
 */
object NativeRdpBridge : RdpInputCallbacks {
    @Volatile private var frameCount: Long = 0
    @Volatile private var running: Boolean = false
    @Volatile private var lastMode: String = "stopped"
    private var inputCoordinateScale: Int = 1
    private val backend: RdpBackend by lazy {
        val go = GomobileRdpBackend()
        if (go.available) go else LoggingRdpBackend()
    }

    fun setInputCoordinateScale(scale: Int) {
        inputCoordinateScale = scale.coerceIn(1, 4)
        Log.i(TAG, "inputCoordinateScale=$inputCoordinateScale")
    }

    fun setCredentials(username: String, password: String) {
        backend.setCredentials(username, password)
    }

    fun startServer(port: Int, hasProjection: Boolean) {
        backend.setInputCallbacks(this)
        backend.startServer(port)
        frameCount = 0
        running = true
        lastMode = if (hasProjection) "screen capture" else "test pattern / no projection"
        Log.i(TAG, "startServer(port=$port, hasProjection=$hasProjection, backend=${backend.name})")
    }

    fun submitFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray) {
        frameCount += 1
        if (frameCount == 1L || frameCount % 120L == 0L) {
            Log.i(TAG, "frame#$frameCount ${width}x$height pixelStride=$pixelStride rowStride=$rowStride bytes=${data.size} backend=${backend.name}")
        }
        backend.submitFrame(width, height, pixelStride, rowStride, data)
    }

    override fun onPointerMove(x: Int, y: Int) {
        RdpAccessibilityService.handlePointerMove(x * inputCoordinateScale, y * inputCoordinateScale)
    }

    override fun onPointerButton(x: Int, y: Int, buttons: Int, down: Boolean) {
        RdpAccessibilityService.handlePointerButton(x * inputCoordinateScale, y * inputCoordinateScale, buttons, down)
    }

    override fun onPointerWheel(x: Int, y: Int, delta: Int, horizontal: Boolean) {
        RdpAccessibilityService.handlePointerWheel(x * inputCoordinateScale, y * inputCoordinateScale, delta, horizontal)
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
        RdpAccessibilityService.handleTouchContact(contactId, x * inputCoordinateScale, y * inputCoordinateScale, flags)
    }

    override fun onTouchFrameEnd() {
        RdpAccessibilityService.handleTouchFrameEnd()
    }

    fun stopServer() {
        backend.stopServer()
        running = false
        lastMode = "stopped"
        Log.i(TAG, "stopServer(backend=${backend.name})")
    }

    fun healthStatus(): String = "backend=${backend.name}, running=$running, mode=$lastMode, frames=$frameCount, inputScale=$inputCoordinateScale"

    private const val TAG = "GoRdpAndroid"
}
