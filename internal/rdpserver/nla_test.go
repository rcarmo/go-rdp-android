package rdpserver

import (
	"bytes"
	"net"
	"testing"

	rdpauth "github.com/rcarmo/go-rdp/pkg/auth"
)

func TestPerformCredSSPRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	pubKey := []byte("test tls subject public key")

	resultCh := make(chan ClientInfo, 1)
	errCh := make(chan error, 1)
	go func() {
		info, err := performCredSSP(server, StaticCredentials{Username: "rui", Password: "secret"}, pubKey)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- info
	}()

	ntlm := rdpauth.NewNTLMv2("", "rui", "secret")
	clientNonce := bytes.Repeat([]byte{0x42}, 32)
	if _, err := client.Write(rdpauth.EncodeTSRequestWithNonce([][]byte{ntlm.GetNegotiateMessage()}, nil, nil, clientNonce)); err != nil {
		t.Fatal(err)
	}

	challengeBytes, err := readCredSSPMessage(client)
	if err != nil {
		t.Fatal(err)
	}
	challengeReq, err := rdpauth.DecodeTSRequest(challengeBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(challengeReq.NegoTokens) != 1 {
		t.Fatalf("challenge tokens = %d, want 1", len(challengeReq.NegoTokens))
	}
	authMsg, sec := ntlm.GetAuthenticateMessage(challengeReq.NegoTokens[0].Data)
	if authMsg == nil || sec == nil {
		t.Fatal("client failed to build auth message")
	}
	clientPubKeyAuth := sec.GssEncrypt(rdpauth.ComputeClientPubKeyAuth(challengeReq.Version, pubKey, clientNonce))
	if _, err := client.Write(rdpauth.EncodeTSRequestWithNonce([][]byte{authMsg}, nil, clientPubKeyAuth, clientNonce)); err != nil {
		t.Fatal(err)
	}

	serverPubKeyBytes, err := readCredSSPMessage(client)
	if err != nil {
		t.Fatal(err)
	}
	serverPubKeyReq, err := rdpauth.DecodeTSRequest(serverPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	serverPubKeyAuth := sec.GssDecrypt(serverPubKeyReq.PubKeyAuth)
	if !bytes.Equal(serverPubKeyAuth, rdpauth.ComputeServerPubKeyAuth(serverPubKeyReq.Version, pubKey, clientNonce)) {
		t.Fatalf("server pubKeyAuth mismatch")
	}

	creds := rdpauth.EncodeCredentials(utf16ForTest(""), utf16ForTest("rui"), utf16ForTest("secret"))
	if _, err := client.Write(rdpauth.EncodeTSRequest(nil, sec.GssEncrypt(creds), nil)); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errCh:
		t.Fatal(err)
	case info := <-resultCh:
		if info.UserName != "rui" || info.Password != "secret" {
			t.Fatalf("unexpected NLA info: %#v", info)
		}
	}
}

func TestMatchClientPubKeyAuthCandidates(t *testing.T) {
	nonce := bytes.Repeat([]byte{0x31}, 32)
	primary := []byte("subject-public-key")
	fallback := []byte("subject-public-key-info")
	actual := rdpauth.ComputeClientPubKeyAuth(6, fallback, nonce)
	matched, ok := matchClientPubKeyAuth(6, [][]byte{primary, fallback}, [][]byte{nonce}, actual)
	if !ok || !bytes.Equal(matched.PublicKey, fallback) || matched.Order != credSSPHashNonceThenKey {
		t.Fatalf("expected fallback public key match, got ok=%v matched=%#v", ok, matched)
	}
	if _, ok := matchClientPubKeyAuth(6, [][]byte{primary}, [][]byte{nonce}, actual); ok {
		t.Fatal("unexpected match")
	}
}

func TestMatchClientPubKeyAuthAlternateHashOrder(t *testing.T) {
	nonce := bytes.Repeat([]byte{0x32}, 32)
	pubKey := []byte("subject-public-key")
	actual := computeCredSSPPubKeyHash(rdpauth.ClientServerHashMagic, pubKey, nonce, credSSPHashKeyThenNonce)
	matched, ok := matchClientPubKeyAuth(6, [][]byte{pubKey}, [][]byte{nonce}, actual)
	if !ok || !bytes.Equal(matched.PublicKey, pubKey) || matched.Order != credSSPHashKeyThenNonce {
		t.Fatalf("expected alternate hash-order match, got ok=%v matched=%#v", ok, matched)
	}
	serverAuth := computeServerPubKeyAuthForBinding(6, matched)
	want := computeCredSSPPubKeyHash(rdpauth.ServerClientHashMagic, pubKey, nonce, credSSPHashKeyThenNonce)
	if !bytes.Equal(serverAuth, want) {
		t.Fatal("server pubKeyAuth should preserve matched hash order")
	}
}

func TestMatchClientPubKeyAuthServerNonce(t *testing.T) {
	clientNonce := bytes.Repeat([]byte{0x33}, 32)
	serverNonce := bytes.Repeat([]byte{0x44}, 32)
	pubKey := []byte("subject-public-key")
	actual := rdpauth.ComputeClientPubKeyAuth(6, pubKey, serverNonce)
	matched, ok := matchClientPubKeyAuth(6, [][]byte{pubKey}, [][]byte{clientNonce, serverNonce}, actual)
	if !ok || !bytes.Equal(matched.Nonce, serverNonce) {
		t.Fatalf("expected server nonce match, got ok=%v matched=%#v", ok, matched)
	}
}

func TestMatchClientPubKeyAuthDefensiveVariants(t *testing.T) {
	clientNonce := bytes.Repeat([]byte{0x55}, 32)
	serverNonce := bytes.Repeat([]byte{0x66}, 32)
	pubKey := []byte("subject-public-key")

	cases := []struct {
		name    string
		version int
		nonce   []byte
		order   string
		wantOK  bool
	}{
		{name: "standard-client-nonce", version: 6, nonce: clientNonce, order: credSSPHashNonceThenKey, wantOK: true},
		{name: "standard-server-nonce", version: 6, nonce: serverNonce, order: credSSPHashNonceThenKey, wantOK: true},
		{name: "alternate-client-nonce", version: 6, nonce: clientNonce, order: credSSPHashKeyThenNonce, wantOK: true},
		{name: "alternate-server-nonce", version: 6, nonce: serverNonce, order: credSSPHashKeyThenNonce, wantOK: true},
		{name: "alternate-v4-rejected", version: 4, nonce: clientNonce, order: credSSPHashKeyThenNonce, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var actual []byte
			if tc.order == credSSPHashKeyThenNonce {
				actual = computeCredSSPPubKeyHash(rdpauth.ClientServerHashMagic, pubKey, tc.nonce, credSSPHashKeyThenNonce)
			} else {
				actual = rdpauth.ComputeClientPubKeyAuth(tc.version, pubKey, tc.nonce)
			}
			matched, ok := matchClientPubKeyAuth(tc.version, [][]byte{pubKey}, [][]byte{clientNonce, serverNonce}, actual)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want=%v matched=%#v", ok, tc.wantOK, matched)
			}
			if !tc.wantOK {
				return
			}
			if !bytes.Equal(matched.PublicKey, pubKey) || !bytes.Equal(matched.Nonce, tc.nonce) || matched.Order != tc.order {
				t.Fatalf("unexpected match %#v", matched)
			}
		})
	}
}

func TestCompactPublicKeyCandidates(t *testing.T) {
	candidates := compactPublicKeyCandidates([][]byte{nil, []byte("a"), []byte("a"), []byte("b")})
	if len(candidates) != 2 || !bytes.Equal(candidates[0], []byte("a")) || !bytes.Equal(candidates[1], []byte("b")) {
		t.Fatalf("unexpected candidates: %q", candidates)
	}
}

func TestPerformCredSSPRejectsBadPassword(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	errCh := make(chan error, 1)
	go func() {
		_, err := performCredSSP(server, StaticCredentials{Username: "rui", Password: "secret"}, []byte("pubkey"))
		errCh <- err
	}()
	ntlm := rdpauth.NewNTLMv2("", "rui", "wrong")
	clientNonce := bytes.Repeat([]byte{0x24}, 32)
	_, _ = client.Write(rdpauth.EncodeTSRequestWithNonce([][]byte{ntlm.GetNegotiateMessage()}, nil, nil, clientNonce))
	challengeBytes, err := readCredSSPMessage(client)
	if err != nil {
		t.Fatal(err)
	}
	challengeReq, _ := rdpauth.DecodeTSRequest(challengeBytes)
	authMsg, sec := ntlm.GetAuthenticateMessage(challengeReq.NegoTokens[0].Data)
	_, _ = client.Write(rdpauth.EncodeTSRequestWithNonce([][]byte{authMsg}, nil, sec.GssEncrypt(rdpauth.ComputeClientPubKeyAuth(challengeReq.Version, []byte("pubkey"), clientNonce)), clientNonce))
	if err := <-errCh; err == nil {
		t.Fatal("expected CredSSP rejection")
	}
}

func utf16ForTest(s string) []byte {
	return encodeClientInfoString(s)[:len(encodeClientInfoString(s))-2]
}
