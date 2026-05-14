package rdpserver

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var traceEnabled = traceEnabledFromEnv(os.Getenv("GO_RDP_ANDROID_TRACE"), os.Getenv("GO_RDP_ANDROID_LOG_LEVEL"))

func traceEnabledFromEnv(traceEnv, levelEnv string) bool {
	if strings.EqualFold(traceEnv, "1") || strings.EqualFold(traceEnv, "true") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(levelEnv)) {
	case "trace", "debug":
		return true
	default:
		return false
	}
}

func tracef(phase string, format string, args ...any) {
	if !traceEnabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	log.Printf("trace phase=%s %s", phase, msg)
}
