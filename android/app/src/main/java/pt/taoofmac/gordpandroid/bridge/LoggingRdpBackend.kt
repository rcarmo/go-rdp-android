package io.carmo.go.rdp.android.bridge

import android.util.Log
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

class LoggingRdpBackend : RdpBackend {
    override val name: String = "logging-stub"
    override val available: Boolean = true

    private val running = AtomicBoolean(false)
    private val frameCount = AtomicLong(0)
    private var port: Int = 0
    private var callbacks: RdpInputCallbacks? = null

    override fun setInputCallbacks(callbacks: RdpInputCallbacks) {
        this.callbacks = callbacks
    }

    override fun setCredentials(username: String, password: String) {
        Log.i(TAG, "setCredentials(user=${username.ifEmpty { "<empty>" }}, passSet=${password.isNotEmpty()}) [$name]")
    }

    override fun startServer(port: Int) {
        if (running.compareAndSet(false, true)) {
            this.port = port
            frameCount.set(0)
            Log.i(TAG, "startServer(port=$port) [$name]")
        } else {
            Log.i(TAG, "startServer ignored; already running [$name]")
        }
    }

    override fun submitFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray) {
        if (!running.get()) return
        val count = frameCount.incrementAndGet()
        if (count == 1L || count % 120 == 0L) {
            Log.i(TAG, "frame#$count ${width}x$height pixelStride=$pixelStride rowStride=$rowStride bytes=${data.size} [$name]")
        }
    }

    override fun stopServer() {
        if (running.compareAndSet(true, false)) {
            Log.i(TAG, "stopServer(frames=${frameCount.get()}) [$name]")
        }
        port = 0
    }

    override fun listenAddress(): String = if (running.get() && port > 0) ":$port" else ""

    override fun tlsFingerprintSha256(): String = ""

    override fun activeConnections(): Long = 0

    override fun acceptedConnections(): Long = if (running.get()) 0 else 0

    override fun handshakeFailures(): Long = 0

    override fun authFailures(): Long = 0

    override fun inputEvents(): Long = 0

    override fun rdpeiContacts(): Long = 0

    override fun framesSent(): Long = 0

    override fun bitmapBytes(): Long = 0

    override fun dvcFragments(): Long = 0

    override fun submittedFrames(): Long = frameCount.get()

    override fun droppedFrames(): Long = 0

    override fun queuedFrames(): Long = 0

    companion object {
        private const val TAG = "GoRdpAndroid"
    }
}
