package rdpserver

import "testing"

func FuzzParseX224ConnectionRequest(f *testing.F) {
	f.Add([]byte{0x0e, x224TypeConnectionRequest, 0, 0, 0, 1, 0, rdpNegReq, 0, 8, 0, 1, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = parseX224ConnectionRequest(data)
	})
}

func FuzzParseX224Data(f *testing.F) {
	f.Add([]byte{0x02, x224TypeData, 0x80, 0x64, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseX224Data(data)
	})
}

func FuzzParseMCSConnectInitial(f *testing.F) {
	f.Add([]byte{0x7f, 0x65, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseMCSConnectInitial(data)
	})
}

func FuzzParseMCSDomainPDU(f *testing.F) {
	f.Add([]byte{byte(mcsChannelJoinRequestApp << 2), 0, 0, 0x03, 0xeb})
	f.Add([]byte{byte(mcsSendDataRequestApp << 2), 0, 0, 0x03, 0xeb, 0x70, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseMCSDomainPDU(data)
	})
}

func FuzzParseShareControlPDU(f *testing.F) {
	f.Add(buildShareDataPDU(pduType2Synchronize, buildSynchronizePayload()))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseShareControlPDU(data)
	})
}

func FuzzParseConfirmActive(f *testing.F) {
	f.Add(buildTestConfirmActivePDU(defaultShareID, defaultMCSUserID))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseConfirmActive(data)
	})
}

func FuzzParseSlowPathInput(f *testing.F) {
	f.Add(buildSlowPathInputPDU(buildSlowPathInputEvent(slowInputScanCode, 0, 0x1e, 0)))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseSlowPathInput(data)
	})
}

func FuzzParseSecurityPDU(f *testing.F) {
	f.Add([]byte{secInfoPacket, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseSecurityPDU(data)
	})
}
