package io.carmo.go.rdp.android.bridge

interface RdpInputCallbacks {
    fun onPointerMove(x: Int, y: Int)
    fun onPointerButton(x: Int, y: Int, buttons: Int, down: Boolean)
    fun onPointerWheel(x: Int, y: Int, delta: Int, horizontal: Boolean)
    fun onKey(scancode: Int, down: Boolean)
    fun onUnicode(codepoint: Int)
    fun onTouchFrameStart(contactCount: Int)
    fun onTouchContact(contactId: Int, x: Int, y: Int, flags: Int)
    fun onTouchFrameEnd()
}

interface RdpBackend {
    val name: String
    val available: Boolean

    fun setInputCallbacks(callbacks: RdpInputCallbacks)
    fun setCredentials(username: String, password: String)
    fun startServer(port: Int)
    fun submitFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray)
    fun stopServer()
    fun listenAddress(): String
    fun tlsFingerprintSha256(): String
}
