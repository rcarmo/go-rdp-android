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
    private val credentialsConfigured = AtomicBoolean(false)
    @Volatile private var lastMode: String = "stopped"
    @Volatile private var securityMode: String = "negotiate"
    @Volatile private var failedAuthPolicy: String = "limit=5 backoffMs=2000 maxMs=60000"
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
        credentialsConfigured.set(username.isNotBlank() && password.isNotEmpty())
        backend.setCredentials(username, password)
    }

    fun setSecurityMode(mode: String): Boolean {
        val ok = backend.setSecurityMode(mode)
        if (ok) {
            securityMode = mode
        }
        return ok
    }

    fun setFailedAuthPolicy(limit: Int, backoffMs: Int, backoffMaxMs: Int): Boolean {
        val ok = backend.setFailedAuthPolicy(limit, backoffMs, backoffMaxMs)
        if (ok) {
            failedAuthPolicy = "limit=$limit backoffMs=$backoffMs maxMs=$backoffMaxMs"
        }
        return ok
    }

    fun startServer(port: Int, mode: String): Boolean {
        frameCount.set(0)
        running.set(false)
        lastMode = "starting"
        backend.setInputCallbacks(this)
        val started = backend.startServer(port)
        if (started) {
            running.set(true)
            lastMode = mode
            Log.i(TAG, "startServer(port=$port, mode=$mode, backend=${backend.name})")
        } else {
            lastMode = "stopped"
            Log.w(TAG, "startServer failed(port=$port, mode=$mode, backend=${backend.name})")
        }
        return started
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
        frameCount.set(0)
        Log.i(TAG, "stopServer(backend=${backend.name})")
    }

    fun tlsFingerprintSha256(): String = backend.tlsFingerprintSha256()

    fun healthStatus(): String {
        val address = backend.listenAddress().ifEmpty { "n/a" }
        val fingerprint = tlsFingerprintSha256().takeIf { it.isNotEmpty() }?.take(16)?.plus("…") ?: "n/a"
        val input = if (RdpAccessibilityService.isConnected()) "enabled" else "disabled"
        val auth = if (credentialsConfigured.get()) "credentials" else "missing"
        return "backend=${backend.name}, running=${running.get()}, mode=$lastMode, security=$securityMode, failedAuth={$failedAuthPolicy}, auth=$auth, addr=$address, tls=$fingerprint, clients=${backend.activeConnections()}, accepted=${backend.acceptedConnections()}, authFailures=${backend.authFailures()}, handshakeFailures=${backend.handshakeFailures()}, input=$input, inputEvents=${backend.inputEvents()}, rdpeiContacts=${backend.rdpeiContacts()}, dvcFragments=${backend.dvcFragments()}, frames=${frameCount.get()}, sentFrames=${backend.framesSent()}, bitmapBytes=${backend.bitmapBytes()}, queued=${backend.queuedFrames()}, dropped=${backend.droppedFrames()}, inputScale=${inputCoordinateScale.get()}"
    }

    private const val TAG = "GoRdpAndroid"
}
