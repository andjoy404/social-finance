package com.example.social_finance

import android.app.Application

/**
 * Custom Application subclass that ensures the high-importance Android heads-up
 * notification channel exists as soon as the app process starts (including cold
 * background/terminated FCM wakes before MainActivity is ever created).
 */
class SocialFinanceApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        NotificationImageHelper.ensureNotificationChannel(this)
    }
}
