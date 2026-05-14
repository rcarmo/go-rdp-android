package rdpserver

import "testing"

func TestTraceEnabledFromEnv(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		traceEnv string
		levelEnv string
		want     bool
	}{
		{name: "legacy one", traceEnv: "1", want: true},
		{name: "legacy true", traceEnv: "true", want: true},
		{name: "log level trace", levelEnv: "trace", want: true},
		{name: "log level debug", levelEnv: "debug", want: true},
		{name: "log level info", levelEnv: "info", want: false},
		{name: "legacy false", traceEnv: "false", levelEnv: "info", want: false},
		{name: "legacy wins", traceEnv: "true", levelEnv: "info", want: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := traceEnabledFromEnv(tc.traceEnv, tc.levelEnv); got != tc.want {
				t.Fatalf("traceEnabledFromEnv(%q, %q) = %v, want %v", tc.traceEnv, tc.levelEnv, got, tc.want)
			}
		})
	}
}
