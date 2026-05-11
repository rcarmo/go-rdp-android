package io.carmo.go.rdp.android

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.media.projection.MediaProjectionManager
import android.os.Bundle
import android.provider.Settings
import android.text.InputType
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import io.carmo.go.rdp.android.auth.RdpCredentialStore
import io.carmo.go.rdp.android.auth.RdpCredentials
import io.carmo.go.rdp.android.bridge.NativeRdpBridge
import io.carmo.go.rdp.android.service.RdpForegroundService
import io.carmo.go.rdp.android.settings.RdpServerMode
import io.carmo.go.rdp.android.settings.RdpSettingsStore

class MainActivity : Activity() {
    private val projectionRequestCode = 1001
    private var pendingCaptureScale: Int = 1
    private var pendingUsername: String = ""
    private var pendingPassword: String = ""

    private lateinit var credentialStore: RdpCredentialStore
    private lateinit var settingsStore: RdpSettingsStore
    private lateinit var status: TextView
    private lateinit var usernameInput: EditText
    private lateinit var passwordInput: EditText
    private lateinit var captureScaleInput: EditText

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        credentialStore = RdpCredentialStore(this)
        settingsStore = RdpSettingsStore(this)
        val savedSettings = settingsStore.load()

        val autoStartTestPattern = intent?.getBooleanExtra(EXTRA_START_TEST_PATTERN, false) == true
        val autoStartCapture = intent?.getBooleanExtra(EXTRA_START_CAPTURE, false) == true
        val captureScale = intent?.getIntExtra(EXTRA_CAPTURE_SCALE, savedSettings.captureScale)
            ?.coerceIn(RdpSettingsStore.MIN_CAPTURE_SCALE, RdpSettingsStore.MAX_CAPTURE_SCALE)
            ?: savedSettings.captureScale

        val intentUsername = intent?.getStringExtra(EXTRA_USERNAME)?.trim().orEmpty()
        val intentPassword = intent?.getStringExtra(EXTRA_PASSWORD).orEmpty()
        if (intentUsername.isNotEmpty() && intentPassword.isNotEmpty()) {
            credentialStore.save(intentUsername, intentPassword)
        }

        val saved = credentialStore.load()
        val initialUsername = if (intentUsername.isNotEmpty()) intentUsername else saved?.username.orEmpty()
        val initialPassword = if (intentPassword.isNotEmpty()) intentPassword else saved?.password.orEmpty()

        status = TextView(this).apply {
            textSize = 16f
        }
        usernameInput = EditText(this).apply {
            hint = "RDP username"
            setText(initialUsername)
            inputType = InputType.TYPE_CLASS_TEXT
        }
        passwordInput = EditText(this).apply {
            hint = "RDP password"
            setText(initialPassword)
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
        }
        captureScaleInput = EditText(this).apply {
            hint = "Capture scale (1-4)"
            setText(captureScale.toString())
            inputType = InputType.TYPE_CLASS_NUMBER
        }

        val saveCredentials = Button(this).apply {
            text = "Save Credentials"
            setOnClickListener {
                if (saveCredentialsFromInputs(showToast = true)) {
                    updateStatus()
                }
            }
        }
        val accessibility = Button(this).apply {
            text = "Open Accessibility Settings"
            setOnClickListener { startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)) }
        }
        val capture = Button(this).apply {
            text = "Grant Screen Capture"
            setOnClickListener {
                val creds = resolveCredentialsOrWarn() ?: return@setOnClickListener
                requestScreenCapture(resolveCaptureScale(), creds.username, creds.password)
            }
        }
        val testPattern = Button(this).apply {
            text = "Start Test Pattern Server"
            setOnClickListener {
                val creds = resolveCredentialsOrWarn() ?: return@setOnClickListener
                startTestPatternService(creds)
            }
        }
        val stop = Button(this).apply {
            text = "Stop RDP Service"
            setOnClickListener {
                startService(Intent(this@MainActivity, RdpForegroundService::class.java).apply {
                    action = RdpForegroundService.ACTION_STOP
                })
                status.postDelayed({ updateStatus() }, 250)
            }
        }

        setContentView(LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 64, 32, 32)
            addView(status)
            addView(usernameInput)
            addView(passwordInput)
            addView(captureScaleInput)
            addView(saveCredentials)
            addView(accessibility)
            addView(capture)
            addView(testPattern)
            addView(stop)
        })

        updateStatus()

        if (autoStartTestPattern) {
            resolveCredentialsOrWarn()?.let { startTestPatternService(it) }
        }
        if (autoStartCapture) {
            status.post {
                resolveCredentialsOrWarn()?.let { requestScreenCapture(captureScale, it.username, it.password) }
            }
        }
    }

    override fun onResume() {
        super.onResume()
        updateStatus()
    }

    private fun updateStatus() {
        val creds = credentialStore.load()
        val health = NativeRdpBridge.healthStatus()
        status.text = if (creds == null) {
            "Native Android RDP server prototype\n\n1. Set username/password\n2. Enable Accessibility\n3. Grant screen capture\n4. Start service\n\nServer start is blocked until credentials are configured.\n\nHealth: $health"
        } else {
            val settings = settingsStore.load()
            "Native Android RDP server prototype\n\nConfigured user: ${creds.username}\nCapture scale: ${settings.captureScale}x downscale\nLast mode: ${settings.lastMode.name.lowercase().replace('_', ' ')}\n1. Enable Accessibility\n2. Grant screen capture\n3. Start service\n\nHealth: $health"
        }
    }

    private fun saveCredentialsFromInputs(showToast: Boolean): Boolean {
        val username = usernameInput.text?.toString()?.trim().orEmpty()
        val password = passwordInput.text?.toString().orEmpty()
        if (username.isEmpty() || password.isEmpty()) {
            if (showToast) {
                Toast.makeText(this, "Username and password are required", Toast.LENGTH_SHORT).show()
            }
            return false
        }
        credentialStore.save(username, password)
        settingsStore.saveCaptureScale(resolveCaptureScale())
        if (showToast) {
            Toast.makeText(this, "Credentials saved", Toast.LENGTH_SHORT).show()
        }
        return true
    }

    private fun resolveCredentialsOrWarn(): RdpCredentials? {
        val username = usernameInput.text?.toString()?.trim().orEmpty()
        val password = passwordInput.text?.toString().orEmpty()
        if (username.isNotEmpty() && password.isNotEmpty()) {
            return RdpCredentials(username, password)
        }
        val creds = credentialStore.load()
        if (creds != null) {
            return creds
        }
        Toast.makeText(this, "Configure credentials first", Toast.LENGTH_SHORT).show()
        return null
    }

    private fun startTestPatternService(creds: RdpCredentials) {
        val captureScale = resolveCaptureScale()
        settingsStore.save(settingsStore.load().copy(captureScale = captureScale, lastMode = RdpServerMode.TEST_PATTERN))
        val intent = Intent(this, RdpForegroundService::class.java).apply {
            putExtra(RdpForegroundService.EXTRA_TEST_PATTERN, true)
            putExtra(RdpForegroundService.EXTRA_CAPTURE_SCALE, captureScale)
            putExtra(RdpForegroundService.EXTRA_USERNAME, creds.username)
            putExtra(RdpForegroundService.EXTRA_PASSWORD, creds.password)
        }
        startForegroundService(intent)
        status.postDelayed({ updateStatus() }, 250)
    }

    private fun requestScreenCapture(scale: Int, username: String, password: String) {
        pendingCaptureScale = scale.coerceIn(RdpSettingsStore.MIN_CAPTURE_SCALE, RdpSettingsStore.MAX_CAPTURE_SCALE)
        pendingUsername = username
        pendingPassword = password
        val manager = getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        startActivityForResult(manager.createScreenCaptureIntent(), projectionRequestCode)
    }

    @Deprecated("Deprecated in Android framework; adequate for scaffold")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode != projectionRequestCode) return
        val captureScale = pendingCaptureScale
        val username = pendingUsername
        val password = pendingPassword
        pendingCaptureScale = settingsStore.load().captureScale
        pendingUsername = ""
        pendingPassword = ""
        if (resultCode == RESULT_OK && data != null) {
            settingsStore.save(settingsStore.load().copy(captureScale = captureScale, lastMode = RdpServerMode.SCREEN_CAPTURE))
            val intent = Intent(this, RdpForegroundService::class.java).apply {
                putExtra(RdpForegroundService.EXTRA_RESULT_CODE, resultCode)
                putExtra(RdpForegroundService.EXTRA_RESULT_DATA, data)
                putExtra(RdpForegroundService.EXTRA_CAPTURE_SCALE, captureScale)
                putExtra(RdpForegroundService.EXTRA_USERNAME, username)
                putExtra(RdpForegroundService.EXTRA_PASSWORD, password)
            }
            startForegroundService(intent)
            status.postDelayed({ updateStatus() }, 250)
        } else {
            Toast.makeText(this, "Screen capture permission was not granted", Toast.LENGTH_SHORT).show()
            updateStatus()
        }
    }

    private fun resolveCaptureScale(): Int {
        return captureScaleInput.text?.toString()?.toIntOrNull()
            ?.coerceIn(RdpSettingsStore.MIN_CAPTURE_SCALE, RdpSettingsStore.MAX_CAPTURE_SCALE)
            ?: settingsStore.load().captureScale
    }

    companion object {
        const val EXTRA_START_TEST_PATTERN = "start_test_pattern"
        const val EXTRA_START_CAPTURE = "start_capture"
        const val EXTRA_CAPTURE_SCALE = "capture_scale"
        const val EXTRA_USERNAME = "rdp_username"
        const val EXTRA_PASSWORD = "rdp_password"
    }
}
