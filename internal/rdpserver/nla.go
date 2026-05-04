package rdpserver

import (
	"bytes"
	"crypto/rand"
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
	matchedPubKey, ok := matchClientPubKeyAuth(authReq.Version, tlsPublicKeys, clientNonce, clientPubKeyAuth)
	if !ok {
		return zero, fmt.Errorf("client pubKeyAuth binding mismatch")
	}

	serverPubKeyAuth := rdpauth.ComputeServerPubKeyAuth(authReq.Version, matchedPubKey, clientNonce)
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

func matchClientPubKeyAuth(version int, candidates [][]byte, nonce, actual []byte) ([]byte, bool) {
	for _, candidate := range candidates {
		if bytes.Equal(actual, rdpauth.ComputeClientPubKeyAuth(version, candidate, nonce)) {
			return candidate, true
		}
	}
	return nil, false
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
