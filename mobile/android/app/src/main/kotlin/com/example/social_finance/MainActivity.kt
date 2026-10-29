package com.example.social_finance

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.drawable.Icon
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL

object NotificationImageHelper {
    const val CHANNEL_ID = "social_finance_notifications"
    private const val FLUTTER_PREFS_NAME = "FlutterSharedPreferences"
    private const val KEY_LAST_API_URL = "flutter.sf_last_api_url"
    private const val MAX_IMAGE_BYTES = 5 * 1024 * 1024 // 5 MB
    private const val CONNECT_TIMEOUT_MS = 5000
    private const val READ_TIMEOUT_MS = 5000

    fun ensureNotificationChannel(context: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
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
                lockscreenVisibility = Notification.VISIBILITY_PUBLIC
            }

            val notificationManager =
                context.getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
            notificationManager?.createNotificationChannel(channel)
        }
    }

    fun resolveImageUrl(context: Context, rawUrl: String?): String? {
        val trimmed = rawUrl?.trim() ?: return null
        if (trimmed.isEmpty()) return null
        if (trimmed.contains("..") || trimmed.contains("\\")) return null

        val candidate = if (trimmed.startsWith("/")) {
            val prefs = context.getSharedPreferences(FLUTTER_PREFS_NAME, Context.MODE_PRIVATE)
            val savedBase = prefs.getString(KEY_LAST_API_URL, null)?.trim()?.trimEnd('/')
            if (savedBase.isNullOrEmpty()) {
                return null
            }
            "$savedBase$trimmed"
        } else {
            trimmed
        }

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

    fun downloadBitmap(urlString: String): Bitmap? {
        var connection: HttpURLConnection? = null
        return try {
            val url = URL(urlString)
            val proto = url.protocol.lowercase()
            if (proto != "http" && proto != "https") return null

            connection = (url.openConnection() as? HttpURLConnection) ?: return null
            connection.connectTimeout = CONNECT_TIMEOUT_MS
            connection.readTimeout = READ_TIMEOUT_MS
            connection.instanceFollowRedirects = true
            connection.doInput = true
            connection.requestMethod = "GET"
            connection.connect()

            val code = connection.responseCode
            if (code !in 200..299) return null

            val contentLength = connection.contentLength
            if (contentLength > MAX_IMAGE_BYTES) return null

            connection.inputStream.use { input ->
                val buffer = ByteArrayOutputStream()
                val chunk = ByteArray(8192)
                var total = 0
                while (true) {
                    val read = input.read(chunk)
                    if (read == -1) break
                    total += read
                    if (total > MAX_IMAGE_BYTES) return null
                    buffer.write(chunk, 0, read)
                }
                val bytes = buffer.toByteArray()
                if (bytes.isEmpty()) return null
                BitmapFactory.decodeByteArray(bytes, 0, bytes.size)
            }
        } catch (_: Exception) {
            null
        } finally {
            try {
                connection?.disconnect()
            } catch (_: Exception) {
            }
        }
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

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        NotificationImageHelper.ensureNotificationChannel(this)
        requestNotificationPermissionIfNeeded()
        extractNotificationPayload(intent)
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
                        val notificationIdStr = call.argument<String>("notification_id")
                        val eventType = call.argument<String>("type")
                        val sentAt = call.argument<String>("sent_at")
                        val id = call.argument<Int>("id") ?: ((System.currentTimeMillis() % 100000).toInt())
                        showHeadsUpNotification(
                            notificationId = id,
                            title = title,
                            body = body,
                            deepLink = deepLink,
                            imageUrl = imageUrl,
                            notificationIdStr = notificationIdStr,
                            eventType = eventType,
                            sentAt = sentAt
                        )
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

        pendingDeepLink = resolvedLink
        pendingPayload = map

        intent.removeExtra("deep_link")
        intent.removeExtra("notification_id")
    }

    private fun showHeadsUpNotification(
        notificationId: Int,
        title: String,
        body: String,
        deepLink: String,
        imageUrl: String? = null,
        notificationIdStr: String? = null,
        eventType: String? = null,
        sentAt: String? = null
    ) {
        val notificationManager =
            getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager ?: return

        NotificationImageHelper.ensureNotificationChannel(this)
        val resolvedImageUrl = NotificationImageHelper.resolveImageUrl(this, imageUrl)

        val intent = Intent(this, MainActivity::class.java).apply {
            action = ACTION_VIEW_BILL
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra("deep_link", deepLink)
            putExtra("title", title)
            putExtra("body", body)
            if (!resolvedImageUrl.isNullOrBlank()) {
                putExtra("image_url", resolvedImageUrl)
            } else if (!imageUrl.isNullOrBlank()) {
                putExtra("image_url", imageUrl)
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
            this,
            notificationId,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val soundUri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)

        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            @Suppress("DEPRECATION")
            Notification.Builder(this)
        }

        builder.setContentTitle(title)
            .setContentText(body)
            .setSmallIcon(applicationInfo.icon)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .setPriority(Notification.PRIORITY_HIGH)
            .setDefaults(Notification.DEFAULT_ALL)
            .setSound(soundUri)
            .setStyle(Notification.BigTextStyle().bigText(body).setBigContentTitle(title))

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val icon = Icon.createWithResource(this, applicationInfo.icon)
            val action = Notification.Action.Builder(
                icon,
                "Lihat Detail",
                pendingIntent
            ).build()
            builder.addAction(action)
        }

        if (!resolvedImageUrl.isNullOrBlank()) {
            Thread {
                val bitmap = NotificationImageHelper.downloadBitmap(resolvedImageUrl)
                if (bitmap != null) {
                    val bigPictureStyle = Notification.BigPictureStyle()
                        .bigPicture(bitmap)
                        .setBigContentTitle(title)
                        .setSummaryText(body)
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                        bigPictureStyle.bigLargeIcon(null as Icon?)
                    }
                    builder.setLargeIcon(bitmap)
                    builder.setStyle(bigPictureStyle)
                }
                notificationManager.notify(notificationId, builder.build())
            }.start()
        } else {
            notificationManager.notify(notificationId, builder.build())
        }
    }
}
