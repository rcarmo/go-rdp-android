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
        securityMode = prefs.getString(KEY_SECURITY_MODE, RdpSecurityMode.NEGOTIATE.wireValue)?.let { raw ->
            RdpSecurityMode.fromWireValue(raw)
        } ?: RdpSecurityMode.NEGOTIATE,
        failedAuthLimit = prefs.getInt(KEY_FAILED_AUTH_LIMIT, DEFAULT_FAILED_AUTH_LIMIT).coerceIn(MIN_FAILED_AUTH_LIMIT, MAX_FAILED_AUTH_LIMIT),
        failedAuthBackoffMs = prefs.getInt(KEY_FAILED_AUTH_BACKOFF_MS, DEFAULT_FAILED_AUTH_BACKOFF_MS).coerceIn(MIN_FAILED_AUTH_BACKOFF_MS, MAX_FAILED_AUTH_BACKOFF_MS),
        failedAuthBackoffMaxMs = prefs.getInt(KEY_FAILED_AUTH_BACKOFF_MAX_MS, DEFAULT_FAILED_AUTH_BACKOFF_MAX_MS).coerceIn(MIN_FAILED_AUTH_BACKOFF_MS, MAX_FAILED_AUTH_BACKOFF_MS),
    )

    fun save(settings: RdpServerSettings) {
        prefs.edit()
            .putInt(KEY_PORT, settings.port.coerceIn(MIN_PORT, MAX_PORT))
            .putInt(KEY_CAPTURE_SCALE, settings.captureScale.coerceIn(MIN_CAPTURE_SCALE, MAX_CAPTURE_SCALE))
            .putString(KEY_LAST_MODE, settings.lastMode.name)
            .putString(KEY_SECURITY_MODE, settings.securityMode.wireValue)
            .putInt(KEY_FAILED_AUTH_LIMIT, settings.failedAuthLimit.coerceIn(MIN_FAILED_AUTH_LIMIT, MAX_FAILED_AUTH_LIMIT))
            .putInt(KEY_FAILED_AUTH_BACKOFF_MS, settings.failedAuthBackoffMs.coerceIn(MIN_FAILED_AUTH_BACKOFF_MS, MAX_FAILED_AUTH_BACKOFF_MS))
            .putInt(KEY_FAILED_AUTH_BACKOFF_MAX_MS, settings.failedAuthBackoffMaxMs.coerceIn(MIN_FAILED_AUTH_BACKOFF_MS, MAX_FAILED_AUTH_BACKOFF_MS))
            .apply()
    }

    fun saveCaptureScale(captureScale: Int) {
        prefs.edit().putInt(KEY_CAPTURE_SCALE, captureScale.coerceIn(MIN_CAPTURE_SCALE, MAX_CAPTURE_SCALE)).apply()
    }

    fun saveLastMode(mode: RdpServerMode) {
        prefs.edit().putString(KEY_LAST_MODE, mode.name).apply()
    }

    fun saveSecurityMode(mode: RdpSecurityMode) {
        prefs.edit().putString(KEY_SECURITY_MODE, mode.wireValue).apply()
    }

    companion object {
        const val DEFAULT_PORT = 3390
        const val DEFAULT_CAPTURE_SCALE = 1
        const val DEFAULT_FAILED_AUTH_LIMIT = 5
        const val DEFAULT_FAILED_AUTH_BACKOFF_MS = 2_000
        const val DEFAULT_FAILED_AUTH_BACKOFF_MAX_MS = 60_000
        const val MIN_CAPTURE_SCALE = 1
        const val MAX_CAPTURE_SCALE = 4
        const val MIN_FAILED_AUTH_LIMIT = 0
        const val MAX_FAILED_AUTH_LIMIT = 100
        const val MIN_FAILED_AUTH_BACKOFF_MS = 0
        const val MAX_FAILED_AUTH_BACKOFF_MS = 300_000
        private const val MIN_PORT = 1
        private const val MAX_PORT = 65535
        private const val PREFS = "rdp_server_settings"
        private const val KEY_PORT = "port"
        private const val KEY_CAPTURE_SCALE = "capture_scale"
        private const val KEY_LAST_MODE = "last_mode"
        private const val KEY_SECURITY_MODE = "security_mode"
        private const val KEY_FAILED_AUTH_LIMIT = "failed_auth_limit"
        private const val KEY_FAILED_AUTH_BACKOFF_MS = "failed_auth_backoff_ms"
        private const val KEY_FAILED_AUTH_BACKOFF_MAX_MS = "failed_auth_backoff_max_ms"
    }
}

data class RdpServerSettings(
    val port: Int = RdpSettingsStore.DEFAULT_PORT,
    val captureScale: Int = RdpSettingsStore.DEFAULT_CAPTURE_SCALE,
    val lastMode: RdpServerMode = RdpServerMode.NONE,
    val securityMode: RdpSecurityMode = RdpSecurityMode.NEGOTIATE,
    val failedAuthLimit: Int = RdpSettingsStore.DEFAULT_FAILED_AUTH_LIMIT,
    val failedAuthBackoffMs: Int = RdpSettingsStore.DEFAULT_FAILED_AUTH_BACKOFF_MS,
    val failedAuthBackoffMaxMs: Int = RdpSettingsStore.DEFAULT_FAILED_AUTH_BACKOFF_MAX_MS,
)

enum class RdpServerMode {
    NONE,
    TEST_PATTERN,
    SCREEN_CAPTURE,
}

enum class RdpSecurityMode(val wireValue: String, val label: String) {
    NEGOTIATE("negotiate", "Negotiate"),
    RDP_ONLY("rdp-only", "RDP only"),
    TLS_ONLY("tls-only", "TLS only"),
    NLA_REQUIRED("nla-required", "NLA required"),
    ;

    override fun toString(): String = label

    companion object {
        fun fromWireValue(value: String): RdpSecurityMode? = entries.firstOrNull { it.wireValue == value }
    }
}
