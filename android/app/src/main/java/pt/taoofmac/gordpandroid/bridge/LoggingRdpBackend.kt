package pt.taoofmac.gordpandroid.bridge

import android.util.Log
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

class LoggingRdpBackend : RdpBackend {
    override val name: String = "logging-stub"
    override val available: Boolean = true

    private val running = AtomicBoolean(false)
    private val frameCount = AtomicLong(0)
    private var callbacks: RdpInputCallbacks? = null

    override fun setInputCallbacks(callbacks: RdpInputCallbacks) {
        this.callbacks = callbacks
    }

    override fun startServer(port: Int) {
        if (running.compareAndSet(false, true)) {
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
    }

    companion object {
        private const val TAG = "GoRdpAndroid"
    }
}
