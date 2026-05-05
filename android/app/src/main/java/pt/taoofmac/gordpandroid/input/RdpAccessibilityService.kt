package io.carmo.go.rdp.android.input

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.os.SystemClock
import android.util.Log
import android.view.accessibility.AccessibilityEvent
import java.lang.ref.WeakReference
import java.util.concurrent.ConcurrentHashMap

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
        return dispatchPathGesture(path, 50)
    }

    fun dispatchPathGesture(path: Path, durationMs: Long): Boolean {
        val boundedDuration = durationMs.coerceIn(MIN_TOUCH_DURATION_MS, MAX_TOUCH_DURATION_MS)
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(path, 0, boundedDuration))
            .build()
        return dispatchGesture(gesture, null, null)
    }

    companion object {
        private const val TAG = "GoRdpAndroidInput"
        private var activeService: WeakReference<RdpAccessibilityService>? = null
        private const val RDP_SCANCODE_HOME = 0x47
        private const val TOUCH_FLAG_DOWN = 0x1
        private const val TOUCH_FLAG_UPDATE = 0x2
        private const val TOUCH_FLAG_UP = 0x4
        private const val TOUCH_FLAG_CANCELED = 0x20
        private const val MIN_TOUCH_DURATION_MS = 50L
        private const val MAX_TOUCH_DURATION_MS = 1_500L
        private val activeTouches = ConcurrentHashMap<Int, TouchPathState>()

        private data class TouchPathState(
            val path: Path,
            val startedAtMs: Long,
            var lastX: Int,
            var lastY: Int,
            var pointCount: Int = 1,
        )

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
            val service = activeService?.get() ?: return false
            if (down && scancode == RDP_SCANCODE_HOME) {
                val ok = service.performGlobalAction(GLOBAL_ACTION_HOME)
                Log.i(TAG, "globalHome(scancode=$scancode ok=$ok)")
                return ok
            }
            return true
        }

        fun handleUnicode(codepoint: Int): Boolean {
            Log.d(TAG, "unicode(codepoint=$codepoint)")
            return activeService?.get() != null
        }

        fun handleTouchContact(contactId: Int, x: Int, y: Int, flags: Int): Boolean {
            Log.d(TAG, "touchContact(id=$contactId x=$x y=$y flags=$flags)")
            val service = activeService?.get() ?: return false
            val now = SystemClock.uptimeMillis()
            if ((flags and TOUCH_FLAG_DOWN) != 0) {
                activeTouches[contactId] = TouchPathState(
                    path = Path().apply { moveTo(x.toFloat(), y.toFloat()) },
                    startedAtMs = now,
                    lastX = x,
                    lastY = y,
                )
                return true
            }

            val state = activeTouches[contactId]
            if (state == null) {
                Log.w(TAG, "dropping stray touch contact id=$contactId flags=$flags")
                return true
            }

            if ((flags and TOUCH_FLAG_CANCELED) != 0) {
                activeTouches.remove(contactId)
                return true
            }

            if ((flags and TOUCH_FLAG_UPDATE) != 0 || (flags and TOUCH_FLAG_UP) != 0) {
                appendTouchPoint(state, x, y)
            }

            if ((flags and TOUCH_FLAG_UP) != 0) {
                activeTouches.remove(contactId)
                return service.dispatchPathGesture(state.path, now - state.startedAtMs)
            }
            return true
        }

        private fun appendTouchPoint(state: TouchPathState, x: Int, y: Int) {
            if (state.lastX == x && state.lastY == y) return
            state.path.lineTo(x.toFloat(), y.toFloat())
            state.lastX = x
            state.lastY = y
            state.pointCount += 1
        }
    }
}
