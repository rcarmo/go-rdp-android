package rdpserver

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var traceEnabled = strings.EqualFold(os.Getenv("GO_RDP_ANDROID_TRACE"), "1") || strings.EqualFold(os.Getenv("GO_RDP_ANDROID_TRACE"), "true")

func tracef(phase string, format string, args ...any) {
	if !traceEnabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	log.Printf("trace phase=%s %s", phase, msg)
}
