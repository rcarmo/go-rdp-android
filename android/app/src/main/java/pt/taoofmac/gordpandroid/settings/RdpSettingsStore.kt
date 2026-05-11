package io.carmo.go.rdp.android.settings

import android.content.Context

/**
 * Stores non-secret RDP server preferences that are safe to restore after the
 * Android process or Activity is recreated. Credentials remain in
 * RdpCredentialStore so plaintext secrets are not duplicated here.
 */
class RdpSettingsStore(context: Context) {
    private val prefs = context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun load(): RdpServerSettings = RdpServerSettings(
        port = prefs.getInt(KEY_PORT, DEFAULT_PORT).coerceIn(MIN_PORT, MAX_PORT),
        captureScale = prefs.getInt(KEY_CAPTURE_SCALE, DEFAULT_CAPTURE_SCALE).coerceIn(MIN_CAPTURE_SCALE, MAX_CAPTURE_SCALE),
        lastMode = prefs.getString(KEY_LAST_MODE, RdpServerMode.NONE.name)?.let { raw ->
            runCatching { RdpServerMode.valueOf(raw) }.getOrNull()
        } ?: RdpServerMode.NONE,
    )

    fun save(settings: RdpServerSettings) {
        prefs.edit()
            .putInt(KEY_PORT, settings.port.coerceIn(MIN_PORT, MAX_PORT))
            .putInt(KEY_CAPTURE_SCALE, settings.captureScale.coerceIn(MIN_CAPTURE_SCALE, MAX_CAPTURE_SCALE))
            .putString(KEY_LAST_MODE, settings.lastMode.name)
            .apply()
    }

    fun saveCaptureScale(captureScale: Int) {
        prefs.edit().putInt(KEY_CAPTURE_SCALE, captureScale.coerceIn(MIN_CAPTURE_SCALE, MAX_CAPTURE_SCALE)).apply()
    }

    fun saveLastMode(mode: RdpServerMode) {
        prefs.edit().putString(KEY_LAST_MODE, mode.name).apply()
    }

    companion object {
        const val DEFAULT_PORT = 3390
        const val DEFAULT_CAPTURE_SCALE = 1
        const val MIN_CAPTURE_SCALE = 1
        const val MAX_CAPTURE_SCALE = 4
        private const val MIN_PORT = 1
        private const val MAX_PORT = 65535
        private const val PREFS = "rdp_server_settings"
        private const val KEY_PORT = "port"
        private const val KEY_CAPTURE_SCALE = "capture_scale"
        private const val KEY_LAST_MODE = "last_mode"
    }
}

data class RdpServerSettings(
    val port: Int = RdpSettingsStore.DEFAULT_PORT,
    val captureScale: Int = RdpSettingsStore.DEFAULT_CAPTURE_SCALE,
    val lastMode: RdpServerMode = RdpServerMode.NONE,
)

enum class RdpServerMode {
    NONE,
    TEST_PATTERN,
    SCREEN_CAPTURE,
}
