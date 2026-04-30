package pt.taoofmac.gordpandroid.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.IBinder
import pt.taoofmac.gordpandroid.bridge.NativeRdpBridge

class RdpForegroundService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        startForeground(1, notification())
        val resultCode = intent?.getIntExtra(EXTRA_RESULT_CODE, 0) ?: 0
        val hasProjection = intent?.hasExtra(EXTRA_RESULT_DATA) == true && resultCode != 0
        NativeRdpBridge.startServer(3390, hasProjection)
        return START_STICKY
    }

    override fun onDestroy() {
        NativeRdpBridge.stopServer()
        super.onDestroy()
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
