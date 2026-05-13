package io.carmo.go.rdp.android.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.net.ConnectivityManager
import android.net.Network
import android.os.Handler
import android.os.HandlerThread
import android.os.IBinder
import android.util.DisplayMetrics
import android.util.Log
import android.view.WindowManager
import java.net.NetworkInterface
import io.carmo.go.rdp.android.bridge.NativeRdpBridge
import io.carmo.go.rdp.android.capture.ScreenCaptureManager
import io.carmo.go.rdp.android.settings.RdpServerMode
import io.carmo.go.rdp.android.settings.RdpSettingsStore

class RdpForegroundService : Service(), ScreenCaptureManager.Listener {
    private var captureManager: ScreenCaptureManager? = null
    private var testPatternThread: HandlerThread? = null
    private var testPatternHandler: Handler? = null
    private var testPatternFrame = 0
    @Volatile private var activeMode: String = "stopped"
    private var networkCallbackRegistered = false
    private val lifecycleLock = Any()
    private lateinit var settingsStore: RdpSettingsStore
    private lateinit var connectivityManager: ConnectivityManager
    private val networkCallback = object : ConnectivityManager.NetworkCallback() {
        override fun onAvailable(network: Network) = onNetworkChanged("available", network)
        override fun onLost(network: Network) = onNetworkChanged("lost", network)
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createChannel()
        settingsStore = RdpSettingsStore(this)
        connectivityManager = getSystemService(ConnectivityManager::class.java)
        captureManager = ScreenCaptureManager(this, this)
        registerNetworkCallback()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            Log.i(TAG, "Stop requested from foreground notification")
            synchronized(lifecycleLock) {
                settingsStore.saveLastMode(RdpServerMode.NONE)
                captureManager?.stop()
                stopTestPattern()
                NativeRdpBridge.stopServer()
                activeMode = "stopped"
                stopForeground(STOP_FOREGROUND_REMOVE)
            }
            stopSelfResult(startId)
            return START_NOT_STICKY
        }

        val resultCode = intent?.getIntExtra(EXTRA_RESULT_CODE, 0) ?: 0
        val data = intent?.getParcelableExtra<Intent>(EXTRA_RESULT_DATA)
        val testPattern = intent?.getBooleanExtra(EXTRA_TEST_PATTERN, false) == true
        val hasProjection = data != null && resultCode != 0
        val savedSettings = settingsStore.load()
        val captureScale = intent?.getIntExtra(EXTRA_CAPTURE_SCALE, savedSettings.captureScale)
            ?.coerceIn(RdpSettingsStore.MIN_CAPTURE_SCALE, RdpSettingsStore.MAX_CAPTURE_SCALE)
            ?: savedSettings.captureScale
        val username = intent?.getStringExtra(EXTRA_USERNAME)?.trim().orEmpty()
        val password = intent?.getStringExtra(EXTRA_PASSWORD).orEmpty()
        val mode = serviceMode(hasProjection, testPattern)
        if (username.isEmpty() || password.isEmpty()) {
            Log.w(TAG, "Refusing to start RDP server without configured credentials")
            activeMode = "stopped"
            startForeground(NOTIFICATION_ID, notification("missing credentials"))
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelfResult(startId)
            return START_NOT_STICKY
        }
        startForeground(NOTIFICATION_ID, notification(mode))
        synchronized(lifecycleLock) {
            activeMode = mode
            captureManager?.stop()
            stopTestPattern()
            NativeRdpBridge.stopServer()
            val requestedMode = when {
                hasProjection -> RdpServerMode.SCREEN_CAPTURE
                testPattern -> RdpServerMode.TEST_PATTERN
                else -> RdpServerMode.NONE
            }
            NativeRdpBridge.setCredentials(username, password)
            NativeRdpBridge.setInputCoordinateScale(captureScale)
            if (!NativeRdpBridge.startServer(3390, mode)) {
                Log.e(TAG, "Native RDP server failed to start")
                activeMode = "stopped"
                settingsStore.save(savedSettings.copy(lastMode = RdpServerMode.NONE))
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelfResult(startId)
                return START_NOT_STICKY
            }
            settingsStore.save(savedSettings.copy(
                captureScale = captureScale,
                lastMode = requestedMode,
            ))

            when {
                hasProjection && data != null -> {
                    val metrics = currentDisplayMetrics()
                    val captureWidth = (metrics.widthPixels / captureScale).coerceAtLeast(1)
                    val captureHeight = (metrics.heightPixels / captureScale).coerceAtLeast(1)
                    val captureDensity = (metrics.densityDpi / captureScale).coerceAtLeast(1)
                    Log.i(TAG, "Starting MediaProjection capture scale=$captureScale ${captureWidth}x$captureHeight density=$captureDensity")
                    captureManager?.start(resultCode, data, captureWidth, captureHeight, captureDensity, maxFps = 15)
                }
                testPattern -> {
                    Log.i(TAG, "Starting test-pattern frame source")
                    startTestPattern()
                }
                else -> {
                    Log.i(TAG, "RDP server started without projection or test pattern")
                }
            }
        }
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        synchronized(lifecycleLock) {
            captureManager?.stop()
            captureManager = null
            stopTestPattern()
            NativeRdpBridge.stopServer()
            activeMode = "stopped"
        }
        unregisterNetworkCallback()
        super.onDestroy()
    }

    override fun onFrame(width: Int, height: Int, pixelStride: Int, rowStride: Int, data: ByteArray) {
        NativeRdpBridge.submitFrame(width, height, pixelStride, rowStride, data)
    }

    override fun onStopped() {
        Log.i(TAG, "MediaProjection stopped")
        synchronized(lifecycleLock) {
            settingsStore.saveLastMode(RdpServerMode.NONE)
            activeMode = "stopped"
            stopForeground(STOP_FOREGROUND_REMOVE)
        }
        stopSelf()
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

    private fun serviceMode(hasProjection: Boolean, testPattern: Boolean): String = when {
        hasProjection -> "screen capture"
        testPattern -> "test pattern"
        else -> "no frame source"
    }

    private fun registerNetworkCallback() {
        if (networkCallbackRegistered) return
        runCatching {
            connectivityManager.registerDefaultNetworkCallback(networkCallback)
            networkCallbackRegistered = true
        }.onFailure { Log.w(TAG, "register network callback failed", it) }
    }

    private fun unregisterNetworkCallback() {
        if (!networkCallbackRegistered) return
        runCatching { connectivityManager.unregisterNetworkCallback(networkCallback) }
            .onFailure { Log.w(TAG, "unregister network callback failed", it) }
        networkCallbackRegistered = false
    }

    private fun onNetworkChanged(event: String, network: Network) {
        val addresses = localIPv4Addresses().ifEmpty { listOf("no IPv4 address") }.joinToString()
        Log.i(TAG, "Network $event: $network local=$addresses")
        if (activeMode != "stopped") {
            runCatching {
                getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, notification(activeMode, addresses))
            }.onFailure { Log.w(TAG, "network notification refresh failed", it) }
        }
    }

    private fun localIPv4Addresses(): List<String> = runCatching {
        NetworkInterface.getNetworkInterfaces().asSequence()
            .filter { it.isUp && !it.isLoopback }
            .flatMap { it.inetAddresses.asSequence() }
            .map { it.hostAddress.orEmpty() }
            .filter { it.isNotBlank() && !it.contains(':') }
            .toList()
            .sorted()
    }.getOrElse { emptyList() }

    private fun notification(
        mode: String,
        addresses: String = localIPv4Addresses().ifEmpty { listOf("no IPv4 address") }.joinToString(),
    ): Notification {
        val stopIntent = Intent(this, RdpForegroundService::class.java).apply { action = ACTION_STOP }
        val stopPendingIntent = PendingIntent.getService(
            this,
            0,
            stopIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val text = "RDP server: $mode • $addresses"
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("go-rdp-android")
            .setContentText(text)
            .setStyle(Notification.BigTextStyle().bigText(text))
            .setSmallIcon(android.R.drawable.presence_online)
            .setOngoing(true)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "Stop", stopPendingIntent)
            .build()
    }

    companion object {
        const val EXTRA_RESULT_CODE = "result_code"
        const val EXTRA_RESULT_DATA = "result_data"
        const val EXTRA_TEST_PATTERN = "test_pattern"
        const val EXTRA_CAPTURE_SCALE = "capture_scale"
        const val EXTRA_USERNAME = "rdp_username"
        const val EXTRA_PASSWORD = "rdp_password"
        const val ACTION_STOP = "io.carmo.go.rdp.android.service.STOP"
        private const val CHANNEL_ID = "rdp-server"
        private const val NOTIFICATION_ID = 1
        private const val TAG = "GoRdpAndroidService"
    }
}
