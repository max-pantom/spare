package api

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type sessionStore struct {
	mu       sync.Mutex
	codes    map[string]time.Time
	sessions map[string]time.Time
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		codes:    map[string]time.Time{},
		sessions: map[string]time.Time{},
	}
}

func (s *sessionStore) NewCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	code := randomToken(24)
	s.codes[code] = time.Now().Add(time.Minute)
	return code
}

func (s *sessionStore) Exchange(code string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	expires, ok := s.codes[code]
	if !ok || time.Now().After(expires) {
		delete(s.codes, code)
		return "", false
	}
	delete(s.codes, code)
	session := randomToken(32)
	s.sessions[session] = time.Now().Add(12 * time.Hour)
	return session, true
}

func (s *sessionStore) Valid(session string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	expires, ok := s.sessions[session]
	return ok && time.Now().Before(expires)
}

func (s *sessionStore) cleanupLocked() {
	now := time.Now()
	for code, expiry := range s.codes {
		if now.After(expiry) {
			delete(s.codes, code)
		}
	}
	for session, expiry := range s.sessions {
		if now.After(expiry) {
			delete(s.sessions, session)
		}
	}
}

func randomToken(size int) string {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
