package com.example.social_finance

import android.Manifest
import android.app.ActivityManager
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import androidx.core.app.NotificationCompat
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.Callable
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException

data class BitmapDownloadResult(
    val bitmap: Bitmap?,
    val statusCode: Int? = null,
    val bytesLength: Int = 0,
    val failureReason: String? = null
)

object NotificationImageHelper {
    const val CHANNEL_ID = "social_finance_notifications_v2"
    private const val LEGACY_CHANNEL_ID = "social_finance_notifications"
    private const val DEFAULT_OFFICIAL_BASE_URL = "https://api.urena0216.id"
    private const val INTERNAL_NOTIF_IMAGE_PREFIX = "/api/v1/notifications/images/"
    private const val FLUTTER_PREFS_NAME = "FlutterSharedPreferences"
    private const val KEY_LAST_API_URL = "flutter.sf_last_api_url"
    private const val MAX_IMAGE_BYTES = 5 * 1024 * 1024 // 5 MB
    private const val MAX_NOTIFICATION_BITMAP_DIM = 1024 // Keeps IPC Binder payload safe (<1MB)
    private const val CONNECT_TIMEOUT_MS = 4000
    private const val READ_TIMEOUT_MS = 6000
    const val DEFAULT_PRELOAD_TIMEOUT_MS = 1800L
    private const val DEDUP_WINDOW_MS = 8000L

    private val workerExecutor = Executors.newCachedThreadPool()
    private val recentNotifications = object : LinkedHashMap<Int, Long>(32, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<Int, Long>?): Boolean = size > 64
    }

    @Volatile
    var isMainActivityResumed: Boolean = false

    fun isAppInForeground(context: Context): Boolean {
        if (isMainActivityResumed) return true
        return try {
            val am = context.getSystemService(Context.ACTIVITY_SERVICE) as? ActivityManager ?: return false
            val processes = am.runningAppProcesses ?: return false
            val pkg = context.packageName
            processes.any {
                it.processName == pkg &&
                    it.importance == ActivityManager.RunningAppProcessInfo.IMPORTANCE_FOREGROUND
            }
        } catch (_: Exception) {
            false
        }
    }

    fun computeNotificationId(
        messageId: String?,
        notificationIdStr: String?,
        title: String,
        body: String
    ): Int {
        val seed = when {
            !notificationIdStr.isNullOrBlank() -> "nid:${notificationIdStr.trim()}"
            !messageId.isNullOrBlank() -> "mid:${messageId.trim()}"
            else -> "$title:$body"
        }
        val positive = seed.hashCode() and 0x7FFFFFFF
        return if (positive == 0) 1001 else positive
    }

    @Synchronized
    private fun markOrSkipDuplicate(notificationId: Int): Boolean {
        val now = System.currentTimeMillis()
        val previous = recentNotifications[notificationId]
        if (previous != null && (now - previous) < DEDUP_WINDOW_MS) {
            return false
        }
        recentNotifications[notificationId] = now
        return true
    }

    fun ensureNotificationChannel(context: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val notificationManager =
                context.getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
                    ?: return

            val channelName = "Notifikasi Social Finance"
            val channelDescription = "Notifikasi tagihan iuran, status pembayaran, dan pengumuman RT"
            val importance = NotificationManager.IMPORTANCE_HIGH

            val soundUri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)
            val audioAttributes = AudioAttributes.Builder()
                .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                .setUsage(AudioAttributes.USAGE_NOTIFICATION)
                .build()

            val channel = NotificationChannel(CHANNEL_ID, channelName, importance).apply {
                description = channelDescription
                enableLights(true)
                enableVibration(true)
                vibrationPattern = longArrayOf(0, 250, 250, 250)
                setSound(soundUri, audioAttributes)
                setShowBadge(true)
                lockscreenVisibility = Notification.VISIBILITY_PUBLIC
            }

            notificationManager.createNotificationChannel(channel)

            // Clean up legacy channel so devices previously stuck at IMPORTANCE_DEFAULT
            // do not retain duplicate or downgraded channels in Android Settings.
            try {
                if (notificationManager.getNotificationChannel(LEGACY_CHANNEL_ID) != null) {
                    notificationManager.deleteNotificationChannel(LEGACY_CHANNEL_ID)
                }
            } catch (_: Exception) {
                // Safe best-effort cleanup
            }
        }
    }

    fun resolveImageUrl(
        context: Context,
        rawUrl: String?,
        preferredBaseUrl: String? = null
    ): String? {
        var trimmed = rawUrl?.trim() ?: return null
        if (trimmed.isEmpty()) return null
        if (trimmed.contains("..") || trimmed.contains("\\")) return null

        if (trimmed.startsWith("notifications/")) {
            trimmed = "$INTERNAL_NOTIF_IMAGE_PREFIX$trimmed"
        } else if (trimmed.startsWith("api/v1/")) {
            trimmed = "/$trimmed"
        }

        val explicitBase = preferredBaseUrl?.trim()?.trimEnd('/')?.ifEmpty { null }
        val prefs = context.getSharedPreferences(FLUTTER_PREFS_NAME, Context.MODE_PRIVATE)
        val savedBase = prefs.getString(KEY_LAST_API_URL, null)?.trim()?.trimEnd('/')?.ifEmpty { null }
        val fallbackBase = explicitBase ?: savedBase ?: DEFAULT_OFFICIAL_BASE_URL
        val effectiveBase = if (fallbackBase.endsWith("/api/v1")) {
            fallbackBase.removeSuffix("/api/v1")
        } else {
            fallbackBase
        }

        var candidate = if (trimmed.startsWith("/")) {
            "$effectiveBase$trimmed"
        } else {
            val parsed = try {
                Uri.parse(trimmed)
            } catch (_: Exception) {
                null
            }
            val scheme = parsed?.scheme?.lowercase()
            val path = parsed?.path ?: ""
            val idx = path.indexOf(INTERNAL_NOTIF_IMAGE_PREFIX)
            // Only rebase an already-absolute URL when an explicit active base URL
            // was supplied by the active session; never clobber a valid passed URL
            // with a stale SharedPreferences value.
            if (idx >= 0 && explicitBase != null) {
                val cleanExplicit = if (explicitBase.endsWith("/api/v1")) {
                    explicitBase.removeSuffix("/api/v1")
                } else {
                    explicitBase
                }
                "$cleanExplicit${path.substring(idx)}"
            } else if ((scheme == "http" || scheme == "https") && !parsed?.host.isNullOrBlank()) {
                trimmed
            } else if (idx >= 0) {
                "$effectiveBase${path.substring(idx)}"
            } else {
                trimmed
            }
        }
        candidate = candidate.replace(Regex("(/api/v1)(?:/api/v1)+(?=/|$)"), "/api/v1")

        return try {
            val uri = Uri.parse(candidate)
            val scheme = uri.scheme?.lowercase()
            val host = uri.host
            if ((scheme == "http" || scheme == "https") && !host.isNullOrBlank()) {
                candidate
            } else {
                null
            }
        } catch (_: Exception) {
            null
        }
    }

    fun decodeSafeNotificationBitmap(bytes: ByteArray): Bitmap? {
        if (bytes.isEmpty()) return null
        val boundsOpts = BitmapFactory.Options().apply {
            inJustDecodeBounds = true
        }
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, boundsOpts)
        val rawW = boundsOpts.outWidth
        val rawH = boundsOpts.outHeight
        if (rawW <= 0 || rawH <= 0) return null

        var sampleSize = 1
        while ((rawW / (sampleSize * 2)) >= MAX_NOTIFICATION_BITMAP_DIM ||
            (rawH / (sampleSize * 2)) >= MAX_NOTIFICATION_BITMAP_DIM
        ) {
            sampleSize *= 2
        }

        val decodeOpts = BitmapFactory.Options().apply {
            inSampleSize = sampleSize
            inPreferredConfig = Bitmap.Config.ARGB_8888
        }
        val decoded = BitmapFactory.decodeByteArray(bytes, 0, bytes.size, decodeOpts) ?: return null
        val maxSide = maxOf(decoded.width, decoded.height)
        if (maxSide <= MAX_NOTIFICATION_BITMAP_DIM) {
            return decoded
        }
        val scale = MAX_NOTIFICATION_BITMAP_DIM.toFloat() / maxSide.toFloat()
        val targetW = (decoded.width * scale).toInt().coerceAtLeast(1)
        val targetH = (decoded.height * scale).toInt().coerceAtLeast(1)
        val scaled = Bitmap.createScaledBitmap(decoded, targetW, targetH, true)
        if (scaled != decoded) {
            decoded.recycle()
        }
        return scaled
    }

    private fun redactUrlForLog(urlString: String): String {
        return try {
            val uri = Uri.parse(urlString)
            "${uri.scheme}://${uri.host}${if (uri.port != -1) ":${uri.port}" else ""}${uri.path ?: ""}"
        } catch (_: Exception) {
            "<invalid-url>"
        }
    }

    fun downloadBitmapWithStatus(
        urlString: String,
        authToken: String? = null,
        rtId: String? = null
    ): BitmapDownloadResult {
        var connection: HttpURLConnection? = null
        val safeUrl = redactUrlForLog(urlString)
        return try {
            val url = URL(urlString)
            val proto = url.protocol.lowercase()
            if (proto != "http" && proto != "https") {
                return BitmapDownloadResult(null, failureReason = "unsupported_scheme")
            }

            val cleanToken = authToken?.trim()
            val cleanRtId = rtId?.trim()
            android.util.Log.i(
                "SocialFinanceNotif",
                "IMAGE_DOWNLOAD_START: url=$safeUrl has_auth=${!cleanToken.isNullOrEmpty()} has_rt_id=${!cleanRtId.isNullOrEmpty()}"
            )

            connection = (url.openConnection() as? HttpURLConnection)
                ?: return BitmapDownloadResult(null, failureReason = "invalid_connection")
            connection.connectTimeout = CONNECT_TIMEOUT_MS
            connection.readTimeout = READ_TIMEOUT_MS
            connection.instanceFollowRedirects = true
            connection.doInput = true
            connection.requestMethod = "GET"
            connection.setRequestProperty("Accept", "image/*")
            if (!cleanToken.isNullOrEmpty()) {
                connection.setRequestProperty("Authorization", "Bearer $cleanToken")
            }
            if (!cleanRtId.isNullOrEmpty()) {
                connection.setRequestProperty("X-RT-ID", cleanRtId)
            }
            connection.connect()

            val code = connection.responseCode
            if (code !in 200..299) {
                android.util.Log.w(
                    "SocialFinanceNotif",
                    "IMAGE_DOWNLOAD_FAILED: url=$safeUrl http_status=$code"
                )
                return BitmapDownloadResult(null, statusCode = code, failureReason = "http_$code")
            }

            val contentType = connection.contentType?.lowercase()?.trim().orEmpty()
            if (contentType.contains("application/json") || contentType.contains("text/html")) {
                android.util.Log.w(
                    "SocialFinanceNotif",
                    "IMAGE_DOWNLOAD_FAILED: url=$safeUrl invalid_content_type=$contentType"
                )
                return BitmapDownloadResult(null, statusCode = code, failureReason = "non_image_content_type")
            }

            val contentLength = connection.contentLength
            if (contentLength > MAX_IMAGE_BYTES) {
                android.util.Log.w(
                    "SocialFinanceNotif",
                    "IMAGE_DOWNLOAD_FAILED: url=$safeUrl content_too_large=$contentLength"
                )
                return BitmapDownloadResult(null, statusCode = code, failureReason = "too_large")
            }

            connection.inputStream.use { input ->
                val buffer = ByteArrayOutputStream()
                val chunk = ByteArray(8192)
                var total = 0
                while (true) {
                    val read = input.read(chunk)
                    if (read == -1) break
                    total += read
                    if (total > MAX_IMAGE_BYTES) {
                        android.util.Log.w(
                            "SocialFinanceNotif",
                            "IMAGE_DOWNLOAD_FAILED: url=$safeUrl stream_exceeded_max_bytes"
                        )
                        return BitmapDownloadResult(null, statusCode = code, failureReason = "too_large")
                    }
                    buffer.write(chunk, 0, read)
                }
                val bytes = buffer.toByteArray()
                if (bytes.isEmpty()) {
                    android.util.Log.w("SocialFinanceNotif", "IMAGE_DOWNLOAD_FAILED: url=$safeUrl empty_body")
                    return BitmapDownloadResult(null, statusCode = code, failureReason = "empty_body")
                }
                val bitmap = decodeSafeNotificationBitmap(bytes)
                if (bitmap == null) {
                    android.util.Log.w(
                        "SocialFinanceNotif",
                        "IMAGE_DECODE_FAILED: url=$safeUrl bytes=${bytes.size}"
                    )
                    return BitmapDownloadResult(null, statusCode = code, bytesLength = bytes.size, failureReason = "decode_failed")
                }
                android.util.Log.i(
                    "SocialFinanceNotif",
                    "IMAGE_DECODE_OK: url=$safeUrl http_status=$code bytes=${bytes.size} bitmap=${bitmap.width}x${bitmap.height}"
                )
                BitmapDownloadResult(bitmap = bitmap, statusCode = code, bytesLength = bytes.size)
            }
        } catch (e: Exception) {
            android.util.Log.w(
                "SocialFinanceNotif",
                "IMAGE_DOWNLOAD_EXCEPTION: url=$safeUrl type=${e.javaClass.simpleName} msg=${e.message}"
            )
            BitmapDownloadResult(null, failureReason = e.javaClass.simpleName)
        } finally {
            try {
                connection?.disconnect()
            } catch (_: Exception) {
            }
        }
    }

    fun downloadBitmap(
        urlString: String,
        authToken: String? = null,
        rtId: String? = null
    ): Bitmap? = downloadBitmapWithStatus(urlString, authToken, rtId).bitmap

    private fun buildNotification(
        context: Context,
        notificationId: Int,
        title: String,
        body: String,
        deepLink: String,
        resolvedImageUrl: String?,
        rawImageUrl: String?,
        notificationIdStr: String?,
        eventType: String?,
        sentAt: String?,
        bitmap: Bitmap?,
        onlyAlertOnce: Boolean
    ): Notification {
        val intent = Intent(context, MainActivity::class.java).apply {
            action = MainActivity.ACTION_VIEW_BILL
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra("deep_link", deepLink)
            putExtra("title", title)
            putExtra("body", body)
            if (!resolvedImageUrl.isNullOrBlank()) {
                putExtra("image_url", resolvedImageUrl)
            } else if (!rawImageUrl.isNullOrBlank()) {
                putExtra("image_url", rawImageUrl)
            }
            if (!notificationIdStr.isNullOrBlank()) {
                putExtra("notification_id", notificationIdStr)
            }
            if (!eventType.isNullOrBlank()) {
                putExtra("type", eventType)
            }
            if (!sentAt.isNullOrBlank()) {
                putExtra("sent_at", sentAt)
            }
        }

        val pendingIntent = PendingIntent.getActivity(
            context,
            notificationId,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val soundUri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)
        val iconRes = R.drawable.ic_stat_notification

        val builder = NotificationCompat.Builder(context, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(body)
            .setSmallIcon(iconRes)
            .setAutoCancel(true)
            .setOnlyAlertOnce(onlyAlertOnce)
            .setContentIntent(pendingIntent)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setCategory(NotificationCompat.CATEGORY_MESSAGE)
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .setWhen(System.currentTimeMillis())
            .setShowWhen(true)
            .addAction(iconRes, "Lihat Detail", pendingIntent)

        if (!onlyAlertOnce && Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            builder.setDefaults(NotificationCompat.DEFAULT_ALL)
                .setSound(soundUri)
        }

        if (bitmap != null) {
            // Set largeIcon so collapsed heads-up & shade show a crisp right-side photo
            // thumbnail, while bigLargeIcon(null) hides the duplicate thumbnail when
            // expanded so BigPictureStyle renders the full-width preview cleanly.
            val bigPictureStyle = NotificationCompat.BigPictureStyle()
                .bigPicture(bitmap)
                .bigLargeIcon(null as Bitmap?)
                .setBigContentTitle(title)
                .setSummaryText(body)
            builder.setLargeIcon(bitmap)
                .setStyle(bigPictureStyle)
            android.util.Log.i(
                "SocialFinanceNotif",
                "BIG_PICTURE_STYLE_BUILT: id=$notificationId bitmap=${bitmap.width}x${bitmap.height} only_alert_once=$onlyAlertOnce"
            )
        } else {
            builder.setStyle(
                NotificationCompat.BigTextStyle()
                    .bigText(body)
                    .setBigContentTitle(title)
            )
        }

        return builder.build()
    }

    /**
     * Unified notification posting engine used by both foreground (MainActivity) and
     * background (SocialFinanceMessagingService).
     *
     * Strategy for attached images:
     * - If preloaded bytes are provided, decode and post with BigPictureStyle immediately.
     * - If a remote authenticated image URL is provided, start the download off the UI
     *   thread and wait up to [preloadTimeoutMs] (default 1800ms):
     *   1) Fast path: download completes within budget -> post ONE heads-up notification
     *      immediately WITH NotificationCompat.BigPictureStyle + largeIcon so the image
     *      preview is visible on the heads-up popup itself and in the shade.
     *   2) Slow-network / deferred path: budget elapses before download finishes -> post
     *      initial text heads-up promptly (never stalling heads-up indefinitely), keep the
     *      download running in background, and update the EXACT SAME notificationId in-place
     *      with BigPictureStyle + setOnlyAlertOnce(true) once the bitmap arrives (zero duplicates).
     *   3) Failure/timeout/no-image path: gracefully post/keep text notification with no crash.
     */
    fun showNotificationSyncOrAsync(
        context: Context,
        notificationId: Int,
        title: String,
        body: String,
        deepLink: String,
        imageUrl: String? = null,
        authToken: String? = null,
        rtId: String? = null,
        preferredBaseUrl: String? = null,
        imageBytes: ByteArray? = null,
        notificationIdStr: String? = null,
        eventType: String? = null,
        sentAt: String? = null,
        source: String = "foreground",
        preloadTimeoutMs: Long = DEFAULT_PRELOAD_TIMEOUT_MS,
        awaitInitialPost: Boolean = false,
        onComplete: ((Map<String, Any>) -> Unit)? = null
    ): Map<String, Any> {
        val notificationManager =
            context.getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
                ?: return mapOf("status" to "DISPLAY_FAILED", "reason" to "no_notification_manager").also {
                    onComplete?.invoke(it)
                }

        ensureNotificationChannel(context)

        if (!notificationManager.areNotificationsEnabled()) {
            android.util.Log.w("SocialFinanceNotif", "DISPLAY_FAILED: notifications_disabled_for_package source=$source")
            return mapOf("status" to "DISPLAY_FAILED", "reason" to "notifications_disabled").also {
                onComplete?.invoke(it)
            }
        }

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = notificationManager.getNotificationChannel(CHANNEL_ID)
            if (channel != null && channel.importance == NotificationManager.IMPORTANCE_NONE) {
                android.util.Log.w("SocialFinanceNotif", "DISPLAY_FAILED: channel_blocked source=$source")
                return mapOf("status" to "DISPLAY_FAILED", "reason" to "channel_blocked").also {
                    onComplete?.invoke(it)
                }
            }
        }

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                android.util.Log.w("SocialFinanceNotif", "DISPLAY_FAILED: permission_denied source=$source")
                return mapOf("status" to "DISPLAY_FAILED", "reason" to "permission_denied").also {
                    onComplete?.invoke(it)
                }
            }
        }

        if (!markOrSkipDuplicate(notificationId)) {
            android.util.Log.i("SocialFinanceNotif", "DUPLICATE_SUPPRESSED: id=$notificationId source=$source")
            return mapOf(
                "status" to "ANDROID_DISPLAY_REQUESTED",
                "id" to notificationId,
                "channel_id" to CHANNEL_ID,
                "deduplicated" to true
            ).also { onComplete?.invoke(it) }
        }

        val resolvedImageUrl = resolveImageUrl(context, imageUrl, preferredBaseUrl)
        val channelImportance = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            notificationManager.getNotificationChannel(CHANNEL_ID)?.importance
                ?: NotificationManager.IMPORTANCE_HIGH
        } else {
            NotificationManager.IMPORTANCE_HIGH
        }

        android.util.Log.i(
            "SocialFinanceNotif",
            "NOTIFICATION_PIPELINE_START: id=$notificationId source=$source " +
                "has_image_url=${!resolvedImageUrl.isNullOrBlank()} " +
                "image_url=${if (!resolvedImageUrl.isNullOrBlank()) redactUrlForLog(resolvedImageUrl) else "none"} " +
                "channel=$CHANNEL_ID importance=$channelImportance"
        )

        val executeFlow = Runnable {
            try {
                val preloadedBitmap = if (imageBytes != null && imageBytes.isNotEmpty()) {
                    decodeSafeNotificationBitmap(imageBytes)
                } else {
                    null
                }

                if (preloadedBitmap != null || resolvedImageUrl.isNullOrBlank()) {
                    val notification = buildNotification(
                        context = context,
                        notificationId = notificationId,
                        title = title,
                        body = body,
                        deepLink = deepLink,
                        resolvedImageUrl = resolvedImageUrl,
                        rawImageUrl = imageUrl,
                        notificationIdStr = notificationIdStr,
                        eventType = eventType,
                        sentAt = sentAt,
                        bitmap = preloadedBitmap,
                        onlyAlertOnce = false
                    )
                    notificationManager.notify(notificationId, notification)
                    android.util.Log.i(
                        "SocialFinanceNotif",
                        "ANDROID_DISPLAY_POSTED: id=$notificationId source=$source " +
                            "with_image=${preloadedBitmap != null} mode=${if (preloadedBitmap != null) "preloaded_bytes" else "text_only"} " +
                            "channel=$CHANNEL_ID importance=$channelImportance"
                    )
                    val res = mapOf(
                        "status" to "ANDROID_DISPLAY_REQUESTED",
                        "id" to notificationId,
                        "channel_id" to CHANNEL_ID,
                        "channel_importance" to channelImportance,
                        "with_image" to (preloadedBitmap != null)
                    )
                    onComplete?.invoke(res)
                    return@Runnable
                }

                // Remote authenticated image URL present: download off-main-thread with bounded wait.
                val downloadFuture = workerExecutor.submit(Callable {
                    downloadBitmapWithStatus(
                        urlString = resolvedImageUrl,
                        authToken = authToken,
                        rtId = rtId
                    )
                })

                var immediateResult: BitmapDownloadResult? = null
                var timedOutWaitingForPreload = false
                if (preloadTimeoutMs > 0L) {
                    try {
                        immediateResult = downloadFuture.get(preloadTimeoutMs, TimeUnit.MILLISECONDS)
                    } catch (_: TimeoutException) {
                        timedOutWaitingForPreload = true
                    } catch (e: Exception) {
                        immediateResult = BitmapDownloadResult(null, failureReason = e.javaClass.simpleName)
                    }
                } else {
                    timedOutWaitingForPreload = true
                }

                if (!timedOutWaitingForPreload) {
                    val bitmap = immediateResult?.bitmap
                    val notification = buildNotification(
                        context = context,
                        notificationId = notificationId,
                        title = title,
                        body = body,
                        deepLink = deepLink,
                        resolvedImageUrl = resolvedImageUrl,
                        rawImageUrl = imageUrl,
                        notificationIdStr = notificationIdStr,
                        eventType = eventType,
                        sentAt = sentAt,
                        bitmap = bitmap,
                        onlyAlertOnce = false
                    )
                    notificationManager.notify(notificationId, notification)
                    if (bitmap != null) {
                        android.util.Log.i(
                            "SocialFinanceNotif",
                            "ANDROID_DISPLAY_POSTED: id=$notificationId source=$source with_image=true " +
                                "mode=bounded_preload bitmap=${bitmap.width}x${bitmap.height} " +
                                "channel=$CHANNEL_ID importance=$channelImportance"
                        )
                    } else {
                        android.util.Log.w(
                            "SocialFinanceNotif",
                            "ANDROID_DISPLAY_IMAGE_FALLBACK_TEXT: id=$notificationId source=$source " +
                                "reason=${immediateResult?.failureReason ?: "unknown"} channel=$CHANNEL_ID"
                        )
                    }
                    val res = mapOf(
                        "status" to "ANDROID_DISPLAY_REQUESTED",
                        "id" to notificationId,
                        "channel_id" to CHANNEL_ID,
                        "channel_importance" to channelImportance,
                        "with_image" to (bitmap != null)
                    )
                    onComplete?.invoke(res)
                    return@Runnable
                }

                // Preload budget elapsed (or deferred mode requested): post text heads-up immediately
                // so the user is alerted on time, then update the exact same notificationId in-place
                // once the background download finishes.
                val initialTextNotif = buildNotification(
                    context = context,
                    notificationId = notificationId,
                    title = title,
                    body = body,
                    deepLink = deepLink,
                    resolvedImageUrl = resolvedImageUrl,
                    rawImageUrl = imageUrl,
                    notificationIdStr = notificationIdStr,
                    eventType = eventType,
                    sentAt = sentAt,
                    bitmap = null,
                    onlyAlertOnce = false
                )
                notificationManager.notify(notificationId, initialTextNotif)
                android.util.Log.i(
                    "SocialFinanceNotif",
                    "ANDROID_DISPLAY_INITIAL_TEXT: id=$notificationId source=$source " +
                        "reason=image_download_pending channel=$CHANNEL_ID importance=$channelImportance"
                )
                onComplete?.invoke(
                    mapOf(
                        "status" to "ANDROID_DISPLAY_REQUESTED",
                        "id" to notificationId,
                        "channel_id" to CHANNEL_ID,
                        "channel_importance" to channelImportance,
                        "with_image" to false,
                        "image_pending" to true
                    )
                )

                // Continue awaiting download on background worker and enrich in-place with same ID.
                workerExecutor.execute {
                    try {
                        val delayedResult = downloadFuture.get(
                            (CONNECT_TIMEOUT_MS + READ_TIMEOUT_MS + 2000).toLong(),
                            TimeUnit.MILLISECONDS
                        )
                        val bitmap = delayedResult.bitmap
                        if (bitmap != null) {
                            val enrichedNotif = buildNotification(
                                context = context,
                                notificationId = notificationId,
                                title = title,
                                body = body,
                                deepLink = deepLink,
                                resolvedImageUrl = resolvedImageUrl,
                                rawImageUrl = imageUrl,
                                notificationIdStr = notificationIdStr,
                                eventType = eventType,
                                sentAt = sentAt,
                                bitmap = bitmap,
                                onlyAlertOnce = true
                            )
                            notificationManager.notify(notificationId, enrichedNotif)
                            android.util.Log.i(
                                "SocialFinanceNotif",
                                "ANDROID_DISPLAY_UPDATED_BIG_PICTURE: id=$notificationId source=$source " +
                                    "bitmap=${bitmap.width}x${bitmap.height} only_alert_once=true"
                            )
                        } else {
                            android.util.Log.w(
                                "SocialFinanceNotif",
                                "ANDROID_DISPLAY_IMAGE_FALLBACK_TEXT: id=$notificationId source=$source " +
                                    "reason=${delayedResult.failureReason ?: "unknown"}"
                            )
                        }
                    } catch (e: Exception) {
                        android.util.Log.w(
                            "SocialFinanceNotif",
                            "ANDROID_DISPLAY_IMAGE_FALLBACK_TEXT: id=$notificationId source=$source " +
                                "reason=${e.javaClass.simpleName}"
                        )
                    }
                }
            } catch (e: Exception) {
                android.util.Log.e("SocialFinanceNotif", "DISPLAY_FAILED: source=$source msg=${e.message}")
                onComplete?.invoke(
                    mapOf("status" to "DISPLAY_FAILED", "reason" to (e.message ?: "unknown_notify_exception"))
                )
            }
        }

        if (awaitInitialPost) {
            executeFlow.run()
        } else {
            workerExecutor.execute(executeFlow)
        }

        return mapOf(
            "status" to "ANDROID_DISPLAY_REQUESTED",
            "id" to notificationId,
            "channel_id" to CHANNEL_ID,
            "channel_importance" to channelImportance
        )
    }
}

class MainActivity : FlutterActivity() {
    companion object {
        const val CHANNEL_ID = NotificationImageHelper.CHANNEL_ID
        const val METHOD_CHANNEL = "com.example.social_finance/notifications"
        const val ACTION_VIEW_BILL = "com.example.social_finance.VIEW_BILL"
        private const val REQ_POST_NOTIFICATIONS = 1001
    }

    private var methodChannel: MethodChannel? = null
    private var pendingDeepLink: String? = null
    private var pendingPayload: Map<String, String>? = null
    private val mainHandler = Handler(Looper.getMainLooper())

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        NotificationImageHelper.ensureNotificationChannel(this)
        extractNotificationPayload(intent)
    }

    override fun onResume() {
        super.onResume()
        NotificationImageHelper.isMainActivityResumed = true
    }

    override fun onPause() {
        NotificationImageHelper.isMainActivityResumed = false
        super.onPause()
    }

    private fun requestNotificationPermissionIfNeeded() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), REQ_POST_NOTIFICATIONS)
            }
        }
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        methodChannel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, METHOD_CHANNEL).apply {
            setMethodCallHandler { call, result ->
                when (call.method) {
                    "showHeadsUpNotification" -> {
                        val title = call.argument<String>("title") ?: "Notifikasi Baru"
                        val body = call.argument<String>("body") ?: ""
                        val deepLink = call.argument<String>("deep_link") ?: ""
                        val imageUrl = call.argument<String>("image_url")
                        val authToken = call.argument<String>("auth_token")
                        val rtId = call.argument<String>("rt_id")
                        val baseUrl = call.argument<String>("base_url")
                        val imageBytes = call.argument<ByteArray>("image_bytes")
                        val notificationIdStr = call.argument<String>("notification_id")
                        val eventType = call.argument<String>("type")
                        val sentAt = call.argument<String>("sent_at")
                        val preloadTimeoutMs = (call.argument<Number>("preload_timeout_ms")?.toLong())
                            ?: NotificationImageHelper.DEFAULT_PRELOAD_TIMEOUT_MS
                        val id = call.argument<Int>("id")
                            ?: NotificationImageHelper.computeNotificationId(null, notificationIdStr, title, body)

                        if (!authToken.isNullOrBlank()) {
                            NativeNotificationAuthStore.save(applicationContext, authToken, rtId, baseUrl)
                        }

                        var replied = false
                        NotificationImageHelper.showNotificationSyncOrAsync(
                            context = this@MainActivity,
                            notificationId = id,
                            title = title,
                            body = body,
                            deepLink = deepLink,
                            imageUrl = imageUrl,
                            authToken = authToken,
                            rtId = rtId,
                            preferredBaseUrl = baseUrl,
                            imageBytes = imageBytes,
                            notificationIdStr = notificationIdStr,
                            eventType = eventType,
                            sentAt = sentAt,
                            source = "foreground_flutter",
                            preloadTimeoutMs = preloadTimeoutMs,
                            awaitInitialPost = false,
                            onComplete = { displayResult ->
                                mainHandler.post {
                                    if (!replied) {
                                        replied = true
                                        result.success(displayResult)
                                    }
                                }
                            }
                        )
                    }
                    "syncNotificationAuth" -> {
                        val accessToken = call.argument<String>("access_token")
                        val rtId = call.argument<String>("rt_id")
                        val baseUrl = call.argument<String>("base_url")
                        NativeNotificationAuthStore.save(applicationContext, accessToken, rtId, baseUrl)
                        result.success(true)
                    }
                    "clearNotificationAuth" -> {
                        NativeNotificationAuthStore.clear(applicationContext)
                        result.success(true)
                    }
                    "areNotificationsEnabled" -> {
                        result.success(areNotificationsFullyEnabled())
                    }
                    "getNotificationPermissionStatus" -> {
                        result.success(checkNotificationPermissionStatus())
                    }
                    "requestNotificationPermission" -> {
                        requestNotificationPermissionIfNeeded()
                        result.success(true)
                    }
                    "getInitialDeepLink" -> {
                        val link = pendingDeepLink
                        pendingDeepLink = null
                        result.success(link)
                    }
                    "getInitialNotificationPayload" -> {
                        val payload = pendingPayload
                        pendingPayload = null
                        pendingDeepLink = null
                        result.success(payload)
                    }
                    "openExternalUrl" -> {
                        val rawUrl = call.argument<String>("url")
                        val opened = openExternalUrl(rawUrl)
                        result.success(opened)
                    }
                    else -> result.notImplemented()
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        extractNotificationPayload(intent)
        val payload = pendingPayload
        if (payload != null && methodChannel != null) {
            pendingPayload = null
            pendingDeepLink = null
            methodChannel?.invokeMethod("onNotificationTapped", payload)
        }
    }

    private fun extractNotificationPayload(intent: Intent?) {
        val extras = intent?.extras ?: return

        val rawDeepLink = intent.getStringExtra("deep_link")?.trim()
        val notifId = intent.getStringExtra("notification_id")?.trim()
        val type = (intent.getStringExtra("type") ?: intent.getStringExtra("event_type"))?.trim()
        val billId = intent.getStringExtra("bill_id")?.trim()
        val paymentId = intent.getStringExtra("payment_id")?.trim()
        val status = intent.getStringExtra("status")?.trim()
        val title = (intent.getStringExtra("title") ?: extras.getString("gcm.n.title"))?.trim()
        val body = (intent.getStringExtra("body") ?: extras.getString("gcm.n.body"))?.trim()
        val imageUrl = (intent.getStringExtra("image_url") ?: extras.getString("gcm.n.image"))?.trim()
        val sentAt = intent.getStringExtra("sent_at")?.trim()
        val dedupKey = intent.getStringExtra("dedup_key")?.trim()

        var resolvedLink = rawDeepLink
        if (resolvedLink.isNullOrEmpty()) {
            if (!billId.isNullOrEmpty()) {
                resolvedLink = if (!paymentId.isNullOrEmpty()) {
                    "/home/iuran/detail/$billId?payment_id=$paymentId"
                } else {
                    "/home/iuran/detail/$billId"
                }
            } else if (!notifId.isNullOrEmpty()) {
                resolvedLink = "/home/notifikasi/detail/$notifId"
            } else if (!type.isNullOrEmpty() || !title.isNullOrEmpty()) {
                resolvedLink = "/home/notifikasi/detail"
            }
        }

        if (resolvedLink.isNullOrEmpty()) return

        val map = mutableMapOf<String, String>("deep_link" to resolvedLink)
        if (!notifId.isNullOrEmpty()) map["notification_id"] = notifId
        if (!type.isNullOrEmpty()) map["type"] = type
        if (!billId.isNullOrEmpty()) map["bill_id"] = billId
        if (!paymentId.isNullOrEmpty()) map["payment_id"] = paymentId
        if (!status.isNullOrEmpty()) map["status"] = status
        if (!title.isNullOrEmpty()) map["title"] = title
        if (!body.isNullOrEmpty()) map["body"] = body
        if (!imageUrl.isNullOrEmpty()) map["image_url"] = imageUrl
        if (!sentAt.isNullOrEmpty()) map["sent_at"] = sentAt
        if (!dedupKey.isNullOrEmpty()) map["dedup_key"] = dedupKey

        pendingDeepLink = resolvedLink
        pendingPayload = map

        intent.removeExtra("deep_link")
        intent.removeExtra("notification_id")
    }

    private fun openExternalUrl(rawUrl: String?): Boolean {
        val trimmed = rawUrl?.trim() ?: return false
        if (trimmed.isEmpty()) return false
        if (trimmed.any { it.isWhitespace() || it.code < 0x20 || it.code == 0x7F }) return false

        return try {
            val uri = Uri.parse(trimmed)
            val scheme = uri.scheme?.lowercase()
            val host = uri.host
            if ((scheme != "http" && scheme != "https") || host.isNullOrBlank()) {
                return false
            }
            if (host.startsWith(".") || host.endsWith(".") || host.contains("..")) {
                return false
            }
            val browserIntent = Intent(Intent.ACTION_VIEW, uri).apply {
                addCategory(Intent.CATEGORY_BROWSABLE)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            startActivity(browserIntent)
            true
        } catch (_: Exception) {
            false
        }
    }

    private fun checkNotificationPermissionStatus(): String {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) {
                "granted"
            } else {
                "denied"
            }
        } else {
            "not_applicable"
        }
    }

    private fun areNotificationsFullyEnabled(): Boolean {
        val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager ?: return false
        if (!notificationManager.areNotificationsEnabled()) {
            return false
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = notificationManager.getNotificationChannel(CHANNEL_ID)
            if (channel != null && channel.importance == NotificationManager.IMPORTANCE_NONE) {
                return false
            }
        }
        return true
    }
}
