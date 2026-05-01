package pt.taoofmac.gordpandroid.bridge

import android.util.Log
import pt.taoofmac.gordpandroid.input.RdpAccessibilityService

/**
 * Android-facing bridge. It prefers the gomobile-generated Go backend when
 * `mobile.aar` is present under `android/app/libs/`, and falls back to a
 * logging implementation so the app remains buildable in CI before the AAR is generated.
 */
object NativeRdpBridge : RdpInputCallbacks {
    private var frameCount: Long = 0
    private val backend: RdpBackend by lazy {
        val go = GomobileRdpBackend()
        if (go.available) go else LoggingRdpBackend()
    }

    fun startServer(port: Int, hasProjection: Boolean) {
        backend.setInputCallbacks(this)
        backend.startServer(port)
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
        RdpAccessibilityService.handlePointerMove(x, y)
    }

    override fun onPointerButton(x: Int, y: Int, buttons: Int, down: Boolean) {
        RdpAccessibilityService.handlePointerButton(x, y, buttons, down)
    }

    override fun onKey(scancode: Int, down: Boolean) {
        RdpAccessibilityService.handleKey(scancode, down)
    }

    override fun onUnicode(codepoint: Int) {
        RdpAccessibilityService.handleUnicode(codepoint)
    }

    fun stopServer() {
        backend.stopServer()
        Log.i(TAG, "stopServer(backend=${backend.name})")
    }

    private const val TAG = "GoRdpAndroid"
}
