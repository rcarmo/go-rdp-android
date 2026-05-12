package io.carmo.go.rdp.android.bridge

import android.util.Log
import java.lang.reflect.Method
import java.lang.reflect.Proxy

class GomobileRdpBackend : RdpBackend {
    private val mobileClass: Class<*>? = loadClass("mobile.Mobile")
    private val inputHandlerInterface: Class<*>? = loadClass("mobile.InputHandler")

    override val name: String = "gomobile"
    override val available: Boolean = mobileClass != null

    override fun setInputCallbacks(callbacks: RdpInputCallbacks) {
        val cls = mobileClass ?: return
        val iface = inputHandlerInterface ?: return
        val method = findMethod(cls, "setInputHandler", 1) ?: return
        val proxy = Proxy.newProxyInstance(iface.classLoader, arrayOf(iface)) { _, invoked, args ->
            val values = args.orEmpty()
            runCatching {
                when (invoked.name.lowercase()) {
                    "pointermove" -> callbacks.onPointerMove(values.intAt(0), values.intAt(1))
                    "pointerbutton" -> callbacks.onPointerButton(values.intAt(0), values.intAt(1), values.intAt(2), values.boolAt(3))
                    "pointerwheel" -> callbacks.onPointerWheel(values.intAt(0), values.intAt(1), values.intAt(2), values.boolAt(3))
                    "key" -> callbacks.onKey(values.intAt(0), values.boolAt(1))
                    "unicode" -> callbacks.onUnicode(values.intAt(0))
                    "touchframestart" -> callbacks.onTouchFrameStart(values.intAt(0))
                    "touchcontact" -> callbacks.onTouchContact(values.intAt(0), values.intAt(1), values.intAt(2), values.intAt(3))
                    "touchframeend" -> callbacks.onTouchFrameEnd()
                }
            }.onFailure { Log.w(TAG, "input callback ${invoked.name} failed", it) }
            null
        }
        runCatching { method.invoke(null, proxy) }
            .onFailure { Log.w(TAG, "setInputHandler failed", it) }
    }

    override fun setCredentials(username: String, password: String) {
        val method = findMethod(mobileClass ?: return, "setCredentials", 2) ?: return
        invoke(method, username, password)
    }

    override fun startServer(port: Int) {
        val method = findMethod(mobileClass ?: return, "startServer", 1) ?: return
        invoke(method, port)
    }

    override fun submitFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray) {
        val method = findMethod(mobileClass ?: return, "submitFrame", 5) ?: return
        invoke(method, width, height, pixelStride, rowStride, data)
    }

    override fun stopServer() {
        val method = findMethod(mobileClass ?: return, "stopServer", 0) ?: return
        invoke(method)
    }

    override fun listenAddress(): String = callString("addr")

    override fun tlsFingerprintSha256(): String = callString("tlsFingerprintSHA256")

    override fun activeConnections(): Long = callLong("activeConnections")

    override fun acceptedConnections(): Long = callLong("acceptedConnections")

    override fun handshakeFailures(): Long = callLong("handshakeFailures")

    override fun authFailures(): Long = callLong("authFailures")

    override fun submittedFrames(): Long = callLong("submittedFrames")

    override fun droppedFrames(): Long = callLong("droppedFrames")

    override fun queuedFrames(): Long = callLong("queuedFrames")

    private fun callString(methodName: String): String {
        val method = findMethod(mobileClass ?: return "", methodName, 0) ?: return ""
        return runCatching { method.invoke(null) as? String ?: "" }
            .onFailure { Log.w(TAG, "$methodName failed", it) }
            .getOrDefault("")
    }

    private fun callLong(methodName: String): Long {
        val method = findMethod(mobileClass ?: return 0, methodName, 0) ?: return 0
        return runCatching { (method.invoke(null) as? Number)?.toLong() ?: 0 }
            .onFailure { Log.w(TAG, "$methodName failed", it) }
            .getOrDefault(0)
    }

    private fun invoke(method: Method, vararg args: Any) {
        val coerced = method.parameterTypes.mapIndexed { index, type -> coerce(args[index], type) }.toTypedArray()
        runCatching { method.invoke(null, *coerced) }
            .onFailure { Log.w(TAG, "${method.name} failed", it) }
    }

    private fun coerce(value: Any, type: Class<*>): Any = when {
        type == java.lang.Long.TYPE || type == java.lang.Long::class.java -> (value as Number).toLong()
        type == java.lang.Integer.TYPE || type == java.lang.Integer::class.java -> (value as Number).toInt()
        type == java.lang.Boolean.TYPE || type == java.lang.Boolean::class.java -> value as Boolean
        type == ByteArray::class.java -> value as ByteArray
        else -> value
    }

    private fun findMethod(cls: Class<*>, name: String, parameterCount: Int): Method? =
        cls.methods.firstOrNull { it.name.equals(name, ignoreCase = true) && it.parameterCount == parameterCount }

    private fun loadClass(name: String): Class<*>? = runCatching { Class.forName(name) }.getOrNull()

    private fun Array<out Any?>.intAt(index: Int): Int = (this[index] as Number).toInt()
    private fun Array<out Any?>.boolAt(index: Int): Boolean = this[index] as Boolean

    companion object {
        private const val TAG = "GoRdpAndroidGo"
    }
}
