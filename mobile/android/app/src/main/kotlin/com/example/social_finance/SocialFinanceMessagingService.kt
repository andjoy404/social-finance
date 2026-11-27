package com.example.social_finance

import com.google.firebase.messaging.RemoteMessage
import io.flutter.plugins.firebase.messaging.FlutterFirebaseMessagingService

/**
 * Custom FCM messaging service extending FlutterFirebaseMessagingService so
 * token lifecycle callbacks (onNewToken/onRegistered/onUnregistered) remain
 * fully intact while background/terminated data-only RT notifications with
 * private image attachments are fetched with Bearer + X-RT-ID authentication
 * and rendered with NotificationCompat.BigPictureStyle on channel
 * `social_finance_notifications_v2`.
 */
class SocialFinanceMessagingService : FlutterFirebaseMessagingService() {

    override fun onMessageReceived(remoteMessage: RemoteMessage) {
        super.onMessageReceived(remoteMessage)

        val data = remoteMessage.data
        val hasNotifBlock = remoteMessage.notification != null
        val rawImageUrl = data["image_url"]?.trim()
        val msgType = data["type"]?.trim().orEmpty()

        android.util.Log.i(
            "SocialFinanceNotif",
            "FCM_NATIVE_RECEIVED: msg_id=${remoteMessage.messageId ?: "none"} " +
                "has_notif_block=$hasNotifBlock has_image_url=${!rawImageUrl.isNullOrEmpty()} type=$msgType"
        )

        // If the FCM payload includes a `notification` block, Firebase SDK already
        // renders it automatically when in background and Flutter's onMessage handles
        // foreground — skip here to prevent duplicate notifications.
        if (hasNotifBlock) {
            return
        }

        // When the app is actively in the foreground, FlutterFirebaseMessagingReceiver
        // dispatches this data message directly to Dart FirebaseMessaging.onMessage,
        // which invokes showHeadsUpNotification via MainActivity with live session state.
        if (NotificationImageHelper.isAppInForeground(applicationContext)) {
            android.util.Log.i(
                "SocialFinanceNotif",
                "FCM_NATIVE_DELEGATED_FOREGROUND: msg_id=${remoteMessage.messageId ?: "none"}"
            )
            return
        }

        val title = data["title"]?.trim().orEmpty().ifEmpty { "Notifikasi Baru" }
        val body = data["body"]?.trim().orEmpty()
        val notificationIdStr = data["notification_id"]?.trim()
        val deepLink = data["deep_link"]?.trim().orEmpty().ifEmpty {
            if (!notificationIdStr.isNullOrEmpty()) {
                "/home/notifikasi/detail/$notificationIdStr"
            } else {
                "/home/notifikasi/detail"
            }
        }
        val sentAt = data["sent_at"]?.trim()
        val dataRtId = data["rt_id"]?.trim()

        val storedAuth = NativeNotificationAuthStore.read(applicationContext)
        val effectiveRtId = dataRtId?.ifEmpty { null } ?: storedAuth?.rtId
        val notificationId = NotificationImageHelper.computeNotificationId(
            messageId = remoteMessage.messageId,
            notificationIdStr = notificationIdStr,
            title = title,
            body = body
        )

        NotificationImageHelper.showNotificationSyncOrAsync(
            context = applicationContext,
            notificationId = notificationId,
            title = title,
            body = body,
            deepLink = deepLink,
            imageUrl = rawImageUrl,
            authToken = storedAuth?.accessToken,
            rtId = effectiveRtId,
            preferredBaseUrl = storedAuth?.baseUrl,
            imageBytes = null,
            notificationIdStr = notificationIdStr,
            eventType = msgType.ifEmpty { null },
            sentAt = sentAt,
            source = "background_fcm",
            preloadTimeoutMs = NotificationImageHelper.DEFAULT_PRELOAD_TIMEOUT_MS,
            awaitInitialPost = true
        )
    }
}
