package com.example.social_finance

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.graphics.drawable.Icon
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.os.Build
import android.os.Bundle
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    companion object {
        const val CHANNEL_ID = "social_finance_notifications"
        const val METHOD_CHANNEL = "com.example.social_finance/notifications"
        const val ACTION_VIEW_BILL = "com.example.social_finance.VIEW_BILL"
    }

    private var methodChannel: MethodChannel? = null
    private var pendingDeepLink: String? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        createNotificationChannel()
        extractDeepLink(intent)
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
                        val id = call.argument<Int>("id") ?: ((System.currentTimeMillis() % 100000).toInt())
                        showHeadsUpNotification(id, title, body, deepLink)
                        result.success(true)
                    }
                    "getInitialDeepLink" -> {
                        val link = pendingDeepLink
                        pendingDeepLink = null
                        result.success(link)
                    }
                    else -> result.notImplemented()
                }
            }
        }

        // If a deep link was present at launch, deliver it once engine is attached
        pendingDeepLink?.let { link ->
            methodChannel?.invokeMethod("onNotificationTapped", mapOf("deep_link" to link))
            pendingDeepLink = null
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        extractDeepLink(intent)
        pendingDeepLink?.let { link ->
            methodChannel?.invokeMethod("onNotificationTapped", mapOf("deep_link" to link))
            pendingDeepLink = null
        }
    }

    private fun extractDeepLink(intent: Intent?) {
        val link = intent?.getStringExtra("deep_link")
        if (!link.isNullOrBlank()) {
            pendingDeepLink = link
        }
    }

    private fun createNotificationChannel() {
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

            val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
            notificationManager?.createNotificationChannel(channel)
        }
    }

    private fun showHeadsUpNotification(notificationId: Int, title: String, body: String, deepLink: String) {
        val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager ?: return

        createNotificationChannel()

        val intent = Intent(this, MainActivity::class.java).apply {
            action = ACTION_VIEW_BILL
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra("deep_link", deepLink)
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

        // Action button "Lihat Detail"
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val icon = Icon.createWithResource(this, applicationInfo.icon)
            val action = Notification.Action.Builder(
                icon,
                "Lihat Detail",
                pendingIntent
            ).build()
            builder.addAction(action)
        }

        notificationManager.notify(notificationId, builder.build())
    }
}
