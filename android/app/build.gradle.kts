plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "pt.taoofmac.gordpandroid"
    compileSdk = 35

    defaultConfig {
        applicationId = "pt.taoofmac.gordpandroid"
        minSdk = 29
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
    }
}
