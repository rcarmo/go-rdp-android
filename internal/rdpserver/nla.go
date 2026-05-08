package rdpserver

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"time"

	rdpauth "github.com/rcarmo/go-rdp/pkg/auth"
)

const maxCredSSPMessageSize = 64 * 1024

func performCredSSP(conn net.Conn, auth Authenticator, tlsPublicKey []byte) (ClientInfo, error) {
	return performCredSSPWithBindings(conn, auth, [][]byte{tlsPublicKey})
}

func performCredSSPWithBindings(conn net.Conn, auth Authenticator, tlsPublicKeys [][]byte) (ClientInfo, error) {
	var zero ClientInfo
	username, password, ok := staticCredentialPair(auth)
	if !ok {
		return zero, fmt.Errorf("NLA requires StaticCredentials")
	}
	tlsPublicKeys = compactPublicKeyCandidates(tlsPublicKeys)
	if len(tlsPublicKeys) == 0 {
		return zero, fmt.Errorf("NLA requires TLS public key binding")
	}

	first, err := readCredSSPMessage(conn)
	if err != nil {
		return zero, fmt.Errorf("read CredSSP negotiate: %w", err)
	}
	negoReq, err := rdpauth.DecodeTSRequest(first)
	if err != nil {
		return zero, fmt.Errorf("decode CredSSP negotiate: %w", err)
	}
	if len(negoReq.NegoTokens) == 0 {
		return zero, fmt.Errorf("CredSSP negotiate missing NTLM token")
	}
	clientNonce := append([]byte(nil), negoReq.ServerNonce...)

	ntlmServer, err := rdpauth.NewServerNTLMv2("", "GO-RDP-ANDROID")
	if err != nil {
		return zero, err
	}
	challenge, err := ntlmServer.BuildChallengeMessage(negoReq.NegoTokens[0].Data)
	if err != nil {
		return zero, fmt.Errorf("build NTLM challenge: %w", err)
	}
	serverNonce := make([]byte, 32)
	if _, err := rand.Read(serverNonce); err != nil {
		return zero, err
	}
	if _, err := conn.Write(rdpauth.EncodeTSRequestWithVersion(6, [][]byte{challenge}, nil, nil, serverNonce)); err != nil {
		return zero, fmt.Errorf("write CredSSP challenge: %w", err)
	}

	authBytes, err := readCredSSPMessage(conn)
	if err != nil {
		return zero, fmt.Errorf("read CredSSP authenticate: %w", err)
	}
	authReq, err := rdpauth.DecodeTSRequest(authBytes)
	if err != nil {
		return zero, fmt.Errorf("decode CredSSP authenticate: %w", err)
	}
	if len(authReq.NegoTokens) == 0 {
		return zero, fmt.Errorf("CredSSP authenticate missing NTLM token")
	}
	ntlmAuth, security, err := ntlmServer.VerifyAuthenticateMessage(authReq.NegoTokens[0].Data, username, password, "")
	if err != nil {
		return zero, fmt.Errorf("verify NTLM authenticate: %w", err)
	}
	if len(authReq.PubKeyAuth) == 0 {
		return zero, fmt.Errorf("CredSSP authenticate missing pubKeyAuth")
	}
	clientPubKeyAuth := security.GssDecrypt(authReq.PubKeyAuth)
	if clientPubKeyAuth == nil {
		return zero, fmt.Errorf("decrypt client pubKeyAuth")
	}
	matchedBinding, ok := matchClientPubKeyAuth(authReq.Version, tlsPublicKeys, [][]byte{clientNonce, serverNonce}, clientPubKeyAuth)
	if !ok {
		tracef("credssp_pubkeyauth_mismatch", "version=%d client_nonce_len=%d server_nonce_len=%d actual_len=%d candidates=%d", authReq.Version, len(clientNonce), len(serverNonce), len(clientPubKeyAuth), len(tlsPublicKeys))
		return zero, fmt.Errorf("client pubKeyAuth binding mismatch")
	}

	serverPubKeyAuth := computeServerPubKeyAuthForBinding(authReq.Version, matchedBinding)
	if _, err := conn.Write(rdpauth.EncodeTSRequestWithVersion(authReq.Version, nil, nil, security.GssEncrypt(serverPubKeyAuth), nil)); err != nil {
		return zero, fmt.Errorf("write server pubKeyAuth: %w", err)
	}

	credBytes, err := readCredSSPMessage(conn)
	if err != nil {
		return zero, fmt.Errorf("read CredSSP credentials: %w", err)
	}
	credReq, err := rdpauth.DecodeTSRequest(credBytes)
	if err != nil {
		return zero, fmt.Errorf("decode CredSSP credentials: %w", err)
	}
	if len(credReq.AuthInfo) == 0 {
		return zero, fmt.Errorf("CredSSP credentials missing authInfo")
	}
	plainCreds := security.GssDecrypt(credReq.AuthInfo)
	if plainCreds == nil {
		return zero, fmt.Errorf("decrypt CredSSP credentials")
	}
	creds, err := rdpauth.DecodeCredentials(plainCreds)
	if err != nil {
		return zero, fmt.Errorf("decode CredSSP credentials: %w", err)
	}
	info := ClientInfo{Domain: creds.Domain, UserName: creds.Username, Password: creds.Password}
	if info.UserName == "" {
		info.UserName = ntlmAuth.User
	}
	if err := authenticateClientInfo(auth, info); err != nil {
		return zero, err
	}
	return info, nil
}

func compactPublicKeyCandidates(candidates [][]byte) [][]byte {
	out := make([][]byte, 0, len(candidates))
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}
		seen := false
		for _, existing := range out {
			if bytes.Equal(existing, candidate) {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, append([]byte(nil), candidate...))
		}
	}
	return out
}

type credSSPPubKeyBinding struct {
	PublicKey []byte
	Nonce     []byte
	Order     string
}

const (
	credSSPHashNonceThenKey = "nonce-key"
	credSSPHashKeyThenNonce = "key-nonce"
)

func matchClientPubKeyAuth(version int, candidates [][]byte, nonces [][]byte, actual []byte) (credSSPPubKeyBinding, bool) {
	if len(nonces) == 0 {
		nonces = [][]byte{nil}
	}
	for _, candidate := range candidates {
		for _, nonce := range nonces {
			standard := rdpauth.ComputeClientPubKeyAuth(version, candidate, nonce)
			if bytes.Equal(actual, standard) {
				return credSSPPubKeyBinding{PublicKey: candidate, Nonce: append([]byte(nil), nonce...), Order: credSSPHashNonceThenKey}, true
			}
			// Keep a defensive nonce-order fallback for observed interop variance:
			// some clients hash magic||pubKey||nonce instead of magic||nonce||pubKey.
			// We intentionally keep this variant until broad Microsoft-client
			// validation confirms we can prune it safely.
			if version >= 5 && len(nonce) > 0 {
				alternate := computeCredSSPPubKeyHash(rdpauth.ClientServerHashMagic, candidate, nonce, credSSPHashKeyThenNonce)
				if bytes.Equal(actual, alternate) {
					return credSSPPubKeyBinding{PublicKey: candidate, Nonce: append([]byte(nil), nonce...), Order: credSSPHashKeyThenNonce}, true
				}
			}
		}
	}
	return credSSPPubKeyBinding{}, false
}

func computeServerPubKeyAuthForBinding(version int, binding credSSPPubKeyBinding) []byte {
	if binding.Order == credSSPHashKeyThenNonce && version >= 5 && len(binding.Nonce) > 0 {
		return computeCredSSPPubKeyHash(rdpauth.ServerClientHashMagic, binding.PublicKey, binding.Nonce, credSSPHashKeyThenNonce)
	}
	return rdpauth.ComputeServerPubKeyAuth(version, binding.PublicKey, binding.Nonce)
}

func computeCredSSPPubKeyHash(magic, pubKey, nonce []byte, order string) []byte {
	h := sha256.New()
	h.Write(magic)
	if order == credSSPHashKeyThenNonce {
		h.Write(pubKey)
		h.Write(nonce)
	} else {
		h.Write(nonce)
		h.Write(pubKey)
	}
	return h.Sum(nil)
}

func staticCredentialPair(auth Authenticator) (string, string, bool) {
	switch a := auth.(type) {
	case StaticCredentials:
		return a.Username, a.Password, true
	case *StaticCredentials:
		if a == nil {
			return "", "", false
		}
		return a.Username, a.Password, true
	default:
		return "", "", false
	}
}

func readCredSSPMessage(r io.Reader) ([]byte, error) {
	first := make([]byte, 2)
	if _, err := io.ReadFull(r, first); err != nil {
		return nil, err
	}
	if first[0] != 0x30 {
		return nil, fmt.Errorf("unexpected CredSSP tag 0x%02x", first[0])
	}
	length, lenBytes, err := parseDERLength(first[1], r)
	if err != nil {
		return nil, err
	}
	if length > maxCredSSPMessageSize {
		return nil, fmt.Errorf("CredSSP message exceeds %d bytes", maxCredSSPMessageSize)
	}
	out := append([]byte{}, first...)
	out = append(out, lenBytes...)
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	out = append(out, body...)
	return out, nil
}

func parseDERLength(first byte, r io.Reader) (int, []byte, error) {
	if first < 0x80 {
		return int(first), nil, nil
	}
	n := int(first & 0x7f)
	if n == 0 || n > 4 {
		return 0, nil, fmt.Errorf("unsupported DER length byte 0x%02x", first)
	}
	extra := make([]byte, n)
	if _, err := io.ReadFull(r, extra); err != nil {
		return 0, nil, err
	}
	length := 0
	for _, b := range extra {
		length = (length << 8) | int(b)
	}
	return length, extra, nil
}

func withShortReadDeadline(conn net.Conn, d time.Duration) func() {
	_ = conn.SetReadDeadline(time.Now().Add(d))
	return func() { _ = conn.SetReadDeadline(time.Time{}) }
}
