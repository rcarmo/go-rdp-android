Feature: Android RDP remote desktop UX
  The Android RDP server should expose a useful remote desktop session in CI.
  Every scenario must leave both machine-readable evidence and an RDP-rendered screenshot.

  Background:
    Given a Go-backed Android APK is installed on an emulator
    And MediaProjection capture is granted
    And the RDP server is reachable through adb forwarded TCP port 3390

  Scenario: Start a captured RDP desktop session
    When the user starts the RDP capture service
    Then the gomobile backend should start the RDP server
    And the first captured frame should be submitted
    And the RDP probe should produce a home screenshot

  Scenario: Search Android Settings with the keyboard
    When the user opens Settings
    And the user focuses Settings search with the keyboard
    And the user types "wifi"
    Then an RDP screenshot of the Settings search results should be captured

  Scenario: Hit a specific Settings target with a mouse-source tap
    When the user taps the scripted Settings target with a mouse input source
    Then the input validation plan should record the mouse target coordinates
    And an RDP screenshot after the mouse tap should be captured

  Scenario: Reveal notifications with a touchscreen swipe
    When the user swipes down from the top of the Android desktop
    Then the input validation plan should record the swipe coordinates
    And an RDP screenshot of the notification shade gesture result should be captured

  Scenario: Open the browser from the Android home screen
    When the user navigates to the Android home screen
    And the user opens the browser app
    Then the browser should come to the foreground
    And an RDP screenshot of the browser should be captured

  Scenario: Measure performance for all UX scenes
    Then the UX report should include per-scene RDP metrics
    And each scene should have a screenshot section in the PDF report
