package rdpserver

import "testing"

func BenchmarkDefaultDomainParametersSerialize(b *testing.B) {
	params := defaultDomainParameters()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if len(params.serialize()) != len(defaultDomainParametersBER) {
			b.Fatal("bad domain parameter length")
		}
	}
}

func TestBuildGCCConferenceCreateResponse(t *testing.T) {
	resp := buildGCCConferenceCreateResponse(nil)
	if len(resp) == 0 {
		t.Fatal("expected non-empty GCC response")
	}
	// Starts with PER choice 0 and PER object identifier length 5.
	if resp[0] != 0 || resp[1] != 5 {
		t.Fatalf("unexpected GCC prefix: %x", resp[:2])
	}
}

func TestMCSConnectResponseEnvelope(t *testing.T) {
	body := buildGCCConferenceCreateResponse(nil)
	if len(body) == 0 {
		t.Fatal("empty gcc body")
	}
	params := defaultDomainParameters().serialize()
	if len(params) == 0 {
		t.Fatal("empty domain parameters")
	}
}
