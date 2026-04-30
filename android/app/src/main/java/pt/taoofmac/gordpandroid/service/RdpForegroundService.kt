package pt.taoofmac.gordpandroid.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.IBinder
import android.util.DisplayMetrics
import android.util.Log
import android.view.WindowManager
import pt.taoofmac.gordpandroid.bridge.NativeRdpBridge
import pt.taoofmac.gordpandroid.capture.ScreenCaptureManager

class RdpForegroundService : Service(), ScreenCaptureManager.Listener {
    private var captureManager: ScreenCaptureManager? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createChannel()
        captureManager = ScreenCaptureManager(this, this)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        startForeground(1, notification())
        val resultCode = intent?.getIntExtra(EXTRA_RESULT_CODE, 0) ?: 0
        val data = intent?.getParcelableExtra<Intent>(EXTRA_RESULT_DATA)
        val hasProjection = data != null && resultCode != 0
        NativeRdpBridge.startServer(3390, hasProjection)

        if (hasProjection && data != null) {
            val metrics = currentDisplayMetrics()
            captureManager?.start(resultCode, data, metrics.widthPixels, metrics.heightPixels, metrics.densityDpi, maxFps = 15)
        }
        return START_STICKY
    }

    override fun onDestroy() {
        captureManager?.stop()
        captureManager = null
        NativeRdpBridge.stopServer()
        super.onDestroy()
    }

    override fun onFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray) {
        NativeRdpBridge.submitFrame(width, height, pixelStride, rowStride, data)
    }

    override fun onStopped() {
        Log.i("GoRdpAndroid", "MediaProjection stopped")
    }

    private fun currentDisplayMetrics(): DisplayMetrics {
        val metrics = DisplayMetrics()
        @Suppress("DEPRECATION")
        (getSystemService(WINDOW_SERVICE) as WindowManager).defaultDisplay.getRealMetrics(metrics)
        return metrics
    }

    private fun createChannel() {
        val channel = NotificationChannel(CHANNEL_ID, "RDP Server", NotificationManager.IMPORTANCE_LOW)
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    private fun notification(): Notification = Notification.Builder(this, CHANNEL_ID)
        .setContentTitle("go-rdp-android")
        .setContentText("RDP server prototype is running")
        .setSmallIcon(android.R.drawable.presence_online)
        .build()

    companion object {
        const val EXTRA_RESULT_CODE = "result_code"
        const val EXTRA_RESULT_DATA = "result_data"
        private const val CHANNEL_ID = "rdp-server"
    }
}
