# Flutter and plugin proguard rules
-keep class io.flutter.app.** { *; }
-keep class io.flutter.plugin.** { *; }
-keep class io.flutter.util.** { *; }
-keep class io.flutter.view.** { *; }
-keep class io.flutter.** { *; }
-keep class io.flutter.plugins.** { *; }

# SharedPreferences & JNI plugin keep rules for release R8 builds
-dontwarn com.google.android.play.core.**
-dontwarn kotlin.**
-dontwarn kotlinx.**
-dontwarn androidx.security.crypto.**
-dontoptimize


-keep class io.flutter.plugins.sharedpreferences.** { *; }
-keep interface io.flutter.plugins.sharedpreferences.** { *; }
-keep class com.github.dart_lang.jni.** { *; }
-keep class com.it_nomads.fluttersecurestorage.** { *; }
-keep class androidx.security.crypto.** { *; }
-keep class * implements *SharedPreferencesApiRegistrar* { *; }
-keep class **.SharedPreferencesApiRegistrar { *; }

-keep class kotlin.** { *; }
-keep interface kotlin.** { *; }
-keep class kotlin.coroutines.** { *; }
-keep interface kotlin.coroutines.** { *; }
-keep class kotlinx.coroutines.** { *; }
-keep interface kotlinx.coroutines.** { *; }

-keepattributes *Annotation*,Signature,InnerClasses,EnclosingMethod,Exceptions
