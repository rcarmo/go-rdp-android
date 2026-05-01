package pt.taoofmac.gordpandroid.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.Handler
import android.os.HandlerThread
import android.os.IBinder
import android.util.DisplayMetrics
import android.util.Log
import android.view.WindowManager
import pt.taoofmac.gordpandroid.bridge.NativeRdpBridge
import pt.taoofmac.gordpandroid.capture.ScreenCaptureManager

class RdpForegroundService : Service(), ScreenCaptureManager.Listener {
    private var captureManager: ScreenCaptureManager? = null
    private var testPatternThread: HandlerThread? = null
    private var testPatternHandler: Handler? = null
    private var testPatternFrame = 0

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
        val testPattern = intent?.getBooleanExtra(EXTRA_TEST_PATTERN, false) == true
        val hasProjection = data != null && resultCode != 0
        NativeRdpBridge.startServer(3390, hasProjection)

        when {
            hasProjection && data != null -> {
                stopTestPattern()
                val metrics = currentDisplayMetrics()
                captureManager?.start(resultCode, data, metrics.widthPixels, metrics.heightPixels, metrics.densityDpi, maxFps = 15)
            }
            testPattern -> {
                Log.i(TAG, "Starting test-pattern frame source")
                startTestPattern()
            }
            else -> {
                Log.i(TAG, "RDP server started without projection or test pattern")
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        captureManager?.stop()
        captureManager = null
        stopTestPattern()
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

    private fun startTestPattern() {
        if (testPatternThread != null) return
        testPatternFrame = 0
        val thread = HandlerThread("RdpTestPattern").also { it.start() }
        val handler = Handler(thread.looper)
        testPatternThread = thread
        testPatternHandler = handler
        val frameTask = object : Runnable {
            override fun run() {
                val width = 320
                val height = 240
                val data = buildTestPatternFrame(width, height, testPatternFrame++)
                NativeRdpBridge.submitFrame(width, height, 4, width * 4, data)
                handler.postDelayed(this, 200)
            }
        }
        handler.post(frameTask)
    }

    private fun stopTestPattern() {
        testPatternHandler?.removeCallbacksAndMessages(null)
        testPatternHandler = null
        testPatternThread?.quitSafely()
        testPatternThread = null
    }

    private fun buildTestPatternFrame(width: Int, height: Int, frameNo: Int): ByteArray {
        val out = ByteArray(width * height * 4)
        var i = 0
        for (y in 0 until height) {
            for (x in 0 until width) {
                out[i++] = ((x + frameNo * 7) and 0xff).toByte()
                out[i++] = ((y + frameNo * 5) and 0xff).toByte()
                out[i++] = ((x + y + frameNo * 3) and 0xff).toByte()
                out[i++] = 0xff.toByte()
            }
        }
        return out
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
        const val EXTRA_TEST_PATTERN = "test_pattern"
        private const val CHANNEL_ID = "rdp-server"
        private const val TAG = "GoRdpAndroidService"
    }
}
