package com.example.social_finance

import android.content.Context
import android.os.Build
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import org.json.JSONObject
import java.nio.charset.StandardCharsets
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

data class StoredNotificationAuth(
    val accessToken: String,
    val rtId: String?,
    val baseUrl: String?
)

/**
 * Hardware/Keystore-backed AES-GCM encrypted store holding the active user's
 * session token, RT scope, and API base URL strictly for authenticated private
 * notification image fetching when FCM wakes the app in background/terminated
 * states.
 *
 * Security invariants:
 * - Access tokens are NEVER written in plaintext and NEVER logged.
 * - Cleared immediately on logout, unauthorized session reset, or server switch.
 */
object NativeNotificationAuthStore {
    private const val KEYSTORE_PROVIDER = "AndroidKeyStore"
    private const val KEY_ALIAS = "sf_notif_auth_aes_gcm_v1"
    private const val PREFS_NAME = "sf_notif_auth_secure_v1"
    private const val KEY_PAYLOAD = "enc_payload"
    private const val KEY_IV = "enc_iv"
    private const val TRANSFORMATION = "AES/GCM/NoPadding"
    private const val GCM_TAG_BITS = 128

    private fun getOrCreateSecretKey(): SecretKey? {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.M) return null
        return try {
            val ks = KeyStore.getInstance(KEYSTORE_PROVIDER).apply { load(null) }
            val existing = ks.getKey(KEY_ALIAS, null) as? SecretKey
            if (existing != null) return existing

            val keyGen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE_PROVIDER)
            val spec = KeyGenParameterSpec.Builder(
                KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build()
            keyGen.init(spec)
            keyGen.generateKey()
        } catch (e: Exception) {
            android.util.Log.w("SocialFinanceNotif", "AuthStore key init failed: ${e.javaClass.simpleName}")
            null
        }
    }

    @Synchronized
    fun save(context: Context, accessToken: String?, rtId: String?, baseUrl: String?) {
        val cleanToken = accessToken?.trim()
        if (cleanToken.isNullOrEmpty()) {
            clear(context)
            return
        }
        val key = getOrCreateSecretKey() ?: return
        try {
            val json = JSONObject().apply {
                put("access_token", cleanToken)
                val cleanRt = rtId?.trim()
                if (!cleanRt.isNullOrEmpty()) put("rt_id", cleanRt)
                val cleanBase = baseUrl?.trim()?.trimEnd('/')
                if (!cleanBase.isNullOrEmpty()) put("base_url", cleanBase)
            }
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.ENCRYPT_MODE, key)
            val encrypted = cipher.doFinal(json.toString().toByteArray(StandardCharsets.UTF_8))
            val iv = cipher.iv

            context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .edit()
                .putString(KEY_PAYLOAD, Base64.encodeToString(encrypted, Base64.NO_WRAP))
                .putString(KEY_IV, Base64.encodeToString(iv, Base64.NO_WRAP))
                .apply()
        } catch (e: Exception) {
            android.util.Log.w("SocialFinanceNotif", "AuthStore save failed: ${e.javaClass.simpleName}")
        }
    }

    @Synchronized
    fun read(context: Context): StoredNotificationAuth? {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        val encPayload = prefs.getString(KEY_PAYLOAD, null) ?: return null
        val encIv = prefs.getString(KEY_IV, null) ?: return null
        val key = getOrCreateSecretKey() ?: return null
        return try {
            val cipher = Cipher.getInstance(TRANSFORMATION)
            val iv = Base64.decode(encIv, Base64.NO_WRAP)
            cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(GCM_TAG_BITS, iv))
            val decrypted = cipher.doFinal(Base64.decode(encPayload, Base64.NO_WRAP))
            val json = JSONObject(String(decrypted, StandardCharsets.UTF_8))
            val token = json.optString("access_token", "").trim()
            if (token.isEmpty()) return null
            val rtId = json.optString("rt_id", "").trim().ifEmpty { null }
            val baseUrl = json.optString("base_url", "").trim().ifEmpty { null }
            StoredNotificationAuth(accessToken = token, rtId = rtId, baseUrl = baseUrl)
        } catch (e: Exception) {
            android.util.Log.w("SocialFinanceNotif", "AuthStore read failed: ${e.javaClass.simpleName}")
            null
        }
    }

    @Synchronized
    fun clear(context: Context) {
        try {
            context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .edit()
                .clear()
                .apply()
        } catch (_: Exception) {
        }
    }
}
