package pt.taoofmac.gordpandroid.bridge

import android.util.Log

/**
 * Temporary Kotlin stub.
 *
 * This is the seam where a gomobile-generated Go binding should be wired in.
 * Keep the app buildable while the Go RDP server core is still a protocol stub.
 */
object NativeRdpBridge {
    fun startServer(port: Int, hasProjection: Boolean) {
        Log.i("GoRdpAndroid", "startServer(port=$port, hasProjection=$hasProjection) [stub]")
    }

    fun stopServer() {
        Log.i("GoRdpAndroid", "stopServer() [stub]")
    }
}
