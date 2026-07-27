package api

import (
	"testing"
	"time"
)

func TestExpiredBrowserArtifactsAreRejected(t *testing.T) {
	sessions := newSessionStore()
	sessions.codes["expired-code"] = time.Now().Add(-time.Second)
	sessions.sessions["expired-session"] = time.Now().Add(-time.Second)

	if _, ok := sessions.Exchange("expired-code"); ok {
		t.Fatal("expired browser code was accepted")
	}
	if sessions.Valid("expired-session") {
		t.Fatal("expired browser session was accepted")
	}
}
