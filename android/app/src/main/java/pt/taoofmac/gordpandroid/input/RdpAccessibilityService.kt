package io.carmo.go.rdp.android.input

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.util.Log
import android.view.accessibility.AccessibilityEvent
import java.lang.ref.WeakReference

class RdpAccessibilityService : AccessibilityService() {
    override fun onServiceConnected() {
        super.onServiceConnected()
        activeService = WeakReference(this)
        Log.i(TAG, "Accessibility service connected")
    }

    override fun onDestroy() {
        activeService?.clear()
        activeService = null
        super.onDestroy()
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) = Unit
    override fun onInterrupt() = Unit

    fun tap(x: Float, y: Float): Boolean {
        val path = Path().apply { moveTo(x, y) }
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(path, 0, 50))
            .build()
        return dispatchGesture(gesture, null, null)
    }

    companion object {
        private const val TAG = "GoRdpAndroidInput"
        private var activeService: WeakReference<RdpAccessibilityService>? = null

        fun handlePointerMove(x: Int, y: Int): Boolean {
            // Accessibility gesture dispatch has no hover/move equivalent suitable for a cheap MVP.
            Log.d(TAG, "pointerMove($x,$y)")
            return activeService?.get() != null
        }

        fun handlePointerButton(x: Int, y: Int, buttons: Int, down: Boolean): Boolean {
            Log.d(TAG, "pointerButton($x,$y buttons=$buttons down=$down)")
            val service = activeService?.get() ?: return false
            val primaryButton = buttons and 0x1 != 0
            return if (down && primaryButton) service.tap(x.toFloat(), y.toFloat()) else true
        }

        fun handleKey(scancode: Int, down: Boolean): Boolean {
            Log.d(TAG, "key(scancode=$scancode down=$down)")
            return activeService?.get() != null
        }

        fun handleUnicode(codepoint: Int): Boolean {
            Log.d(TAG, "unicode(codepoint=$codepoint)")
            return activeService?.get() != null
        }
    }
}
