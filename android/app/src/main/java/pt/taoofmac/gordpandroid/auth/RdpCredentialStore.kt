package io.carmo.go.rdp.android.auth

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.nio.charset.StandardCharsets
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

data class RdpCredentials(val username: String, val password: String)

class RdpCredentialStore(context: Context) {
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    fun load(): RdpCredentials? {
        val username = prefs.getString(KEY_USERNAME, "")?.trim().orEmpty()
        if (username.isEmpty()) return null

        val encryptedB64 = prefs.getString(KEY_PASSWORD_ENCRYPTED, "").orEmpty()
        val ivB64 = prefs.getString(KEY_PASSWORD_IV, "").orEmpty()
        val password = if (encryptedB64.isNotEmpty() && ivB64.isNotEmpty()) {
            decryptPassword(encryptedB64, ivB64)
        } else {
            // Migrate old plaintext storage to encrypted-at-rest format.
            val legacy = prefs.getString(KEY_PASSWORD_LEGACY, "").orEmpty()
            if (legacy.isNotEmpty()) {
                save(username, legacy)
            }
            legacy
        }

        if (password.isEmpty()) return null
        return RdpCredentials(username, password)
    }

    fun save(username: String, password: String) {
        val normalizedUser = username.trim()
        if (normalizedUser.isEmpty() || password.isEmpty()) {
            clear()
            return
        }
        val encrypted = encryptPassword(password)
        prefs.edit()
            .putString(KEY_USERNAME, normalizedUser)
            .putString(KEY_PASSWORD_ENCRYPTED, encrypted.ciphertextB64)
            .putString(KEY_PASSWORD_IV, encrypted.ivB64)
            .remove(KEY_PASSWORD_LEGACY)
            .apply()
    }

    fun clear() {
        prefs.edit()
            .remove(KEY_USERNAME)
            .remove(KEY_PASSWORD_ENCRYPTED)
            .remove(KEY_PASSWORD_IV)
            .remove(KEY_PASSWORD_LEGACY)
            .apply()
    }

    private fun encryptPassword(password: String): EncryptedValue {
        val key = getOrCreateKey()
        val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key)
        val encrypted = cipher.doFinal(password.toByteArray(StandardCharsets.UTF_8))
        return EncryptedValue(
            ciphertextB64 = Base64.encodeToString(encrypted, Base64.NO_WRAP),
            ivB64 = Base64.encodeToString(cipher.iv, Base64.NO_WRAP),
        )
    }

    private fun decryptPassword(ciphertextB64: String, ivB64: String): String {
        return runCatching {
            val key = getOrCreateKey()
            val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
            val iv = Base64.decode(ivB64, Base64.NO_WRAP)
            val encrypted = Base64.decode(ciphertextB64, Base64.NO_WRAP)
            cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, iv))
            val plain = cipher.doFinal(encrypted)
            String(plain, StandardCharsets.UTF_8)
        }.getOrDefault("")
    }

    private fun getOrCreateKey(): SecretKey {
        val ks = KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }
        val existing = ks.getKey(KEY_ALIAS, null)
        if (existing is SecretKey) return existing

        val keyGenerator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE)
        val spec = KeyGenParameterSpec.Builder(
            KEY_ALIAS,
            KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
        )
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setRandomizedEncryptionRequired(true)
            .build()
        keyGenerator.init(spec)
        return keyGenerator.generateKey()
    }

    private data class EncryptedValue(
        val ciphertextB64: String,
        val ivB64: String,
    )

    companion object {
        private const val PREFS_NAME = "rdp_credentials"
        private const val KEY_USERNAME = "username"
        private const val KEY_PASSWORD_LEGACY = "password"
        private const val KEY_PASSWORD_ENCRYPTED = "password_enc"
        private const val KEY_PASSWORD_IV = "password_iv"

        private const val ANDROID_KEYSTORE = "AndroidKeyStore"
        private const val KEY_ALIAS = "go_rdp_android_credentials"
        private const val CIPHER_TRANSFORMATION = "AES/GCM/NoPadding"
    }
}
