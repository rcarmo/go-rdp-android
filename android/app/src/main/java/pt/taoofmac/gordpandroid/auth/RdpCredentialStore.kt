package io.carmo.go.rdp.android.auth

import android.content.Context

data class RdpCredentials(val username: String, val password: String)

class RdpCredentialStore(context: Context) {
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    fun load(): RdpCredentials? {
        val username = prefs.getString(KEY_USERNAME, "")?.trim().orEmpty()
        val password = prefs.getString(KEY_PASSWORD, "").orEmpty()
        if (username.isEmpty() || password.isEmpty()) return null
        return RdpCredentials(username, password)
    }

    fun save(username: String, password: String) {
        prefs.edit()
            .putString(KEY_USERNAME, username.trim())
            .putString(KEY_PASSWORD, password)
            .apply()
    }

    companion object {
        private const val PREFS_NAME = "rdp_credentials"
        private const val KEY_USERNAME = "username"
        private const val KEY_PASSWORD = "password"
    }
}
