package rdpserver

import "testing"

func TestParseClientInfo(t *testing.T) {
	payload := buildClientInfoPayload("rui", "secret", "home")
	info, err := parseClientInfo(payload)
	if err != nil {
		t.Fatal(err)
	}
	if info.UserName != "rui" || info.Password != "secret" || info.Domain != "home" {
		t.Fatalf("unexpected client info: %#v", info)
	}
}

func TestStaticCredentials(t *testing.T) {
	auth := StaticCredentials{Username: "rui", Password: "secret"}
	if err := authenticateClientInfo(auth, ClientInfo{UserName: "rui", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := authenticateClientInfo(auth, ClientInfo{UserName: "rui", Password: "wrong"}); err == nil {
		t.Fatal("expected invalid credentials")
	}
}
