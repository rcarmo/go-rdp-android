package pt.taoofmac.gordpandroid

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.media.projection.MediaProjectionManager
import android.os.Bundle
import android.provider.Settings
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import pt.taoofmac.gordpandroid.service.RdpForegroundService

class MainActivity : Activity() {
    private val projectionRequestCode = 1001

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val autoStartTestPattern = intent?.getBooleanExtra(EXTRA_START_TEST_PATTERN, false) == true
        val autoStartCapture = intent?.getBooleanExtra(EXTRA_START_CAPTURE, false) == true
        val status = TextView(this).apply {
            text = "Native Android RDP server prototype\n\n1. Enable Accessibility\n2. Grant screen capture\n3. Start service\n\nCI/debug: test-pattern mode can start without MediaProjection."
            textSize = 16f
        }
        val accessibility = Button(this).apply {
            text = "Open Accessibility Settings"
            setOnClickListener { startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)) }
        }
        val capture = Button(this).apply {
            text = "Grant Screen Capture"
            setOnClickListener { requestScreenCapture() }
        }
        val testPattern = Button(this).apply {
            text = "Start Test Pattern Server"
            setOnClickListener { startTestPatternService() }
        }
        val stop = Button(this).apply {
            text = "Stop RDP Service"
            setOnClickListener { stopService(Intent(this@MainActivity, RdpForegroundService::class.java)) }
        }

        setContentView(LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 64, 32, 32)
            addView(status)
            addView(accessibility)
            addView(capture)
            addView(testPattern)
            addView(stop)
        })

        if (autoStartTestPattern) {
            startTestPatternService()
        }
        if (autoStartCapture) {
            status.post { requestScreenCapture() }
        }
    }

    private fun startTestPatternService() {
        val intent = Intent(this, RdpForegroundService::class.java).apply {
            putExtra(RdpForegroundService.EXTRA_TEST_PATTERN, true)
        }
        startService(intent)
    }

    private fun requestScreenCapture() {
        val manager = getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        startActivityForResult(manager.createScreenCaptureIntent(), projectionRequestCode)
    }

    @Deprecated("Deprecated in Android framework; adequate for scaffold")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == projectionRequestCode && resultCode == RESULT_OK && data != null) {
            val intent = Intent(this, RdpForegroundService::class.java).apply {
                putExtra(RdpForegroundService.EXTRA_RESULT_CODE, resultCode)
                putExtra(RdpForegroundService.EXTRA_RESULT_DATA, data)
            }
            startForegroundService(intent)
        }
    }

    companion object {
        const val EXTRA_START_TEST_PATTERN = "start_test_pattern"
        const val EXTRA_START_CAPTURE = "start_capture"
    }
}
