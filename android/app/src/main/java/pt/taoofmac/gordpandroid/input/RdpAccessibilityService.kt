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
        synchronized(touchDispatchLock) {
            activeTouches.clear()
            activePointerGestures.clear()
            inProgressFrameEvents = null
            inProgressFrameExpectedContacts = 0
            dispatchInFlight = false
            dispatchRequested = false
        }
        Log.i(TAG, "Accessibility service connected")
    }

    override fun onDestroy() {
        synchronized(touchDispatchLock) {
            activeTouches.clear()
            activePointerGestures.clear()
            inProgressFrameEvents = null
            inProgressFrameExpectedContacts = 0
            dispatchInFlight = false
            dispatchRequested = false
        }
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
        private const val POINTER_BUTTON_PRIMARY = 0x1
        private const val POINTER_PRIMARY_CONTACT_ID = 1
        private const val MIN_TOUCH_DURATION_MS = 50L
        private const val MAX_TOUCH_DURATION_MS = 1_500L

        private val activeTouches = ConcurrentHashMap<Int, TouchPathState>()
        private val activePointerGestures = ConcurrentHashMap<Int, TouchPathState>()
        private val touchDispatchLock = Any()

        private var inProgressFrameEvents: MutableList<TouchContactEvent>? = null
        private var inProgressFrameExpectedContacts: Int = 0
        private var dispatchInFlight = false
        private var dispatchRequested = false

        private data class TouchPathState(
            var path: Path,
            val startedAtMs: Long,
            var segmentStartedAtMs: Long,
            var lastX: Int,
            var lastY: Int,
            var pointCount: Int = 1,
            var dirty: Boolean = true,
            var ending: Boolean = false,
            var previousStroke: GestureDescription.StrokeDescription? = null,
        )

        private data class TouchContactEvent(
            val contactId: Int,
            val x: Int,
            val y: Int,
            val flags: Int,
        )

        private data class DispatchedTouchStroke(
            val contactId: Int,
            val continuing: Boolean,
            val descriptor: GestureDescription.StrokeDescription,
        )

        fun isConnected(): Boolean = activeService?.get() != null

        fun handlePointerMove(x: Int, y: Int): Boolean {
            Log.d(TAG, "pointerMove($x,$y)")
            if (activeService?.get() == null) return false
            activePointerGestures[POINTER_PRIMARY_CONTACT_ID]?.let { appendTouchPoint(it, x, y) }
            return true
        }

        fun handlePointerButton(x: Int, y: Int, buttons: Int, down: Boolean): Boolean {
            Log.d(TAG, "pointerButton($x,$y buttons=$buttons down=$down)")
            val service = activeService?.get() ?: return false
            val primaryButton = (buttons and POINTER_BUTTON_PRIMARY) != 0
            if (!primaryButton) return true
            val now = SystemClock.uptimeMillis()
            if (down) {
                activePointerGestures[POINTER_PRIMARY_CONTACT_ID] = TouchPathState(
                    path = Path().apply { moveTo(x.toFloat(), y.toFloat()) },
                    startedAtMs = now,
                    segmentStartedAtMs = now,
                    lastX = x,
                    lastY = y,
                )
                return true
            }
            val state = activePointerGestures.remove(POINTER_PRIMARY_CONTACT_ID)
            if (state == null) {
                Log.w(TAG, "dropping stray primary pointer up at $x,$y")
                return true
            }
            appendTouchPoint(state, x, y)
            return service.dispatchPathGesture(state.path, now - state.startedAtMs)
        }

        fun handlePointerWheel(x: Int, y: Int, delta: Int, horizontal: Boolean): Boolean {
            Log.d(TAG, "pointerWheel($x,$y delta=$delta horizontal=$horizontal)")
            // AccessibilityService has no reliable generic wheel injection primitive.
            // Keep the event visible in diagnostics and degrade safely.
            return activeService?.get() != null
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

        fun handleTouchFrameStart(contactCount: Int): Boolean {
            val serviceConnected = activeService?.get() != null
            synchronized(touchDispatchLock) {
                inProgressFrameExpectedContacts = contactCount.coerceAtLeast(0)
                inProgressFrameEvents = if (serviceConnected) ArrayList(inProgressFrameExpectedContacts) else null
            }
            return serviceConnected
        }

        fun handleTouchContact(contactId: Int, x: Int, y: Int, flags: Int): Boolean {
            Log.d(TAG, "touchContact(id=$contactId x=$x y=$y flags=$flags)")
            val service = activeService?.get() ?: return false
            synchronized(touchDispatchLock) {
                val event = TouchContactEvent(contactId = contactId, x = x, y = y, flags = flags)
                val frameEvents = inProgressFrameEvents
                if (frameEvents != null) {
                    frameEvents.add(event)
                    return true
                }
                processTouchEventsLocked(listOf(event), SystemClock.uptimeMillis())
                return scheduleTouchDispatchLocked(service)
            }
        }

        fun handleTouchFrameEnd(): Boolean {
            val service = activeService?.get()
            synchronized(touchDispatchLock) {
                val frameEvents = inProgressFrameEvents ?: return service != null
                if (service == null) {
                    inProgressFrameExpectedContacts = 0
                    inProgressFrameEvents = null
                    return false
                }
                val expectedContacts = inProgressFrameExpectedContacts
                inProgressFrameExpectedContacts = 0
                inProgressFrameEvents = null
                if (expectedContacts > 0 && frameEvents.size != expectedContacts) {
                    Log.w(TAG, "touch frame contact mismatch expected=$expectedContacts actual=${frameEvents.size}")
                }
                processTouchEventsLocked(frameEvents, SystemClock.uptimeMillis())
                return scheduleTouchDispatchLocked(service)
            }
        }

        private fun processTouchEventsLocked(events: List<TouchContactEvent>, nowMs: Long) {
            for (event in events) {
                val isDown = (event.flags and TOUCH_FLAG_DOWN) != 0
                val isUpdate = (event.flags and TOUCH_FLAG_UPDATE) != 0
                val isUp = (event.flags and TOUCH_FLAG_UP) != 0
                val isCanceled = (event.flags and TOUCH_FLAG_CANCELED) != 0

                if (isDown) {
                    activeTouches[event.contactId] = TouchPathState(
                        path = Path().apply { moveTo(event.x.toFloat(), event.y.toFloat()) },
                        startedAtMs = nowMs,
                        segmentStartedAtMs = nowMs,
                        lastX = event.x,
                        lastY = event.y,
                    )
                }

                val state = activeTouches[event.contactId]
                if (state == null) {
                    Log.w(TAG, "dropping stray touch contact id=${event.contactId} flags=${event.flags}")
                    continue
                }

                if (isCanceled) {
                    activeTouches.remove(event.contactId)
                    continue
                }

                if (isDown || isUpdate || isUp) {
                    appendTouchPoint(state, event.x, event.y)
                    state.dirty = true
                }
                if (isUp) {
                    state.ending = true
                }
            }
        }

        private fun scheduleTouchDispatchLocked(service: RdpAccessibilityService): Boolean {
            if (dispatchInFlight) {
                dispatchRequested = true
                return true
            }
            val dispatchAtMs = SystemClock.uptimeMillis()
            val built = buildTouchDispatchBatchLocked(dispatchAtMs)
            if (built.isEmpty()) {
                return true
            }
            if (dispatchTouchStrokesLocked(service, built, dispatchAtMs)) {
                return true
            }
            return handleFailedTouchDispatchLocked(service, built, dispatchAtMs)
        }

        private fun dispatchTouchStrokesLocked(
            service: RdpAccessibilityService,
            strokes: List<DispatchedTouchStroke>,
            dispatchAtMs: Long,
        ): Boolean {
            val gestureBuilder = GestureDescription.Builder()
            strokes.forEach { gestureBuilder.addStroke(it.descriptor) }
            val gesture = gestureBuilder.build()

            dispatchInFlight = true
            dispatchRequested = false
            val callback = object : AccessibilityService.GestureResultCallback() {
                override fun onCompleted(gestureDescription: GestureDescription?) {
                    finishTouchDispatch(strokes, canceled = false)
                }

                override fun onCancelled(gestureDescription: GestureDescription?) {
                    finishTouchDispatch(strokes, canceled = true)
                }
            }

            val dispatched = service.dispatchGesture(gesture, callback, null)
            if (!dispatched) {
                dispatchInFlight = false
                return false
            }
            commitTouchDispatchLocked(strokes, dispatchAtMs)
            return true
        }

        private fun commitTouchDispatchLocked(strokes: List<DispatchedTouchStroke>, dispatchAtMs: Long) {
            val endingContactIDs = ArrayList<Int>()
            for (stroke in strokes) {
                val state = activeTouches[stroke.contactId] ?: continue
                state.previousStroke = if (stroke.continuing) stroke.descriptor else null
                state.path = Path().apply { moveTo(state.lastX.toFloat(), state.lastY.toFloat()) }
                state.segmentStartedAtMs = dispatchAtMs
                state.dirty = false
                if (!stroke.continuing || state.ending) {
                    endingContactIDs += stroke.contactId
                }
            }
            endingContactIDs.forEach(activeTouches::remove)
        }

        private fun handleFailedTouchDispatchLocked(
            service: RdpAccessibilityService,
            attempted: List<DispatchedTouchStroke>,
            dispatchAtMs: Long,
        ): Boolean {
            attempted.filter { it.continuing }.forEach { stroke ->
                activeTouches[stroke.contactId]?.previousStroke = null
            }

            if (attempted.size > 1) {
                val primary = attempted.first()
                val dropped = attempted.drop(1)
                dropped.forEach { activeTouches.remove(it.contactId) }
                Log.w(
                    TAG,
                    "multi-touch dispatch unsupported; falling back to single contact id=${primary.contactId} dropped=${dropped.size}",
                )
                if (dispatchTouchStrokesLocked(service, listOf(primary), dispatchAtMs)) {
                    return true
                }
            }

            markDroppedTouchDispatchLocked(attempted, dispatchAtMs)
            if (dispatchRequested || hasDirtyTouchStateLocked()) {
                dispatchRequested = false
                return scheduleTouchDispatchLocked(service)
            }
            return false
        }

        private fun markDroppedTouchDispatchLocked(strokes: List<DispatchedTouchStroke>, dispatchAtMs: Long) {
            val endingContactIDs = ArrayList<Int>()
            for (stroke in strokes) {
                val state = activeTouches[stroke.contactId] ?: continue
                state.previousStroke = null
                state.path = Path().apply { moveTo(state.lastX.toFloat(), state.lastY.toFloat()) }
                state.segmentStartedAtMs = dispatchAtMs
                state.dirty = false
                if (!stroke.continuing || state.ending) {
                    endingContactIDs += stroke.contactId
                }
            }
            endingContactIDs.forEach(activeTouches::remove)
        }

        private fun finishTouchDispatch(dispatched: List<DispatchedTouchStroke>, canceled: Boolean) {
            synchronized(touchDispatchLock) {
                dispatchInFlight = false
                if (canceled) {
                    dispatched.filter { it.continuing }.forEach { stroke ->
                        activeTouches[stroke.contactId]?.previousStroke = null
                    }
                }
                val service = activeService?.get()
                if (service != null && (dispatchRequested || hasDirtyTouchStateLocked())) {
                    dispatchRequested = false
                    scheduleTouchDispatchLocked(service)
                } else {
                    dispatchRequested = false
                }
            }
        }

        private fun buildTouchDispatchBatchLocked(nowMs: Long): List<DispatchedTouchStroke> {
            if (activeTouches.isEmpty()) {
                return emptyList()
            }
            val out = ArrayList<DispatchedTouchStroke>(activeTouches.size)
            activeTouches.entries.sortedBy { it.key }.forEach { (contactId, state) ->
                if (!state.dirty && !state.ending) return@forEach

                val durationMs = (nowMs - state.segmentStartedAtMs).coerceIn(MIN_TOUCH_DURATION_MS, MAX_TOUCH_DURATION_MS)
                val continueAfter = !state.ending
                val segmentPath = Path(state.path)
                val descriptor = runCatching {
                    state.previousStroke?.continueStroke(segmentPath, 0, durationMs, continueAfter)
                        ?: GestureDescription.StrokeDescription(segmentPath, 0, durationMs, continueAfter)
                }.getOrElse {
                    GestureDescription.StrokeDescription(segmentPath, 0, durationMs, continueAfter)
                }

                out += DispatchedTouchStroke(contactId = contactId, continuing = continueAfter, descriptor = descriptor)
            }
            return out
        }

        private fun hasDirtyTouchStateLocked(): Boolean = activeTouches.values.any { it.dirty || it.ending }

        private fun appendTouchPoint(state: TouchPathState, x: Int, y: Int) {
            if (state.lastX == x && state.lastY == y) return
            state.path.lineTo(x.toFloat(), y.toFloat())
            state.lastX = x
            state.lastY = y
            state.pointCount += 1
        }
    }
}
