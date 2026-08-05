package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaptureListAndInspectRequest(t *testing.T) {
	hook := newServer()
	handler := hook.routes()

	request := httptest.NewRequest(http.MethodPost, "/hook/orders?source=stripe", strings.NewReader(`{"paid":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Webhook-Signature", "secret")
	request.RemoteAddr = "192.168.1.44:52100"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("capture status = %d body=%s", response.Code, response.Body.String())
	}
	id := response.Header().Get("X-Spare-Hook-Request-ID")
	if id == "" {
		t.Fatal("capture response did not include a request id")
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/requests", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d", response.Code)
	}
	var listed struct {
		Requests []requestSummary `json:"requests"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Requests) != 1 {
		t.Fatalf("request count = %d", len(listed.Requests))
	}
	summary := listed.Requests[0]
	if summary.ID != id || summary.Method != http.MethodPost || summary.Path != "/hook/orders" ||
		summary.Query != "source=stripe" || summary.RemoteAddress != "192.168.1.44" {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/requests/"+id, nil))
	var captured capturedRequest
	if err := json.Unmarshal(response.Body.Bytes(), &captured); err != nil {
		t.Fatal(err)
	}
	if captured.Body != `{"paid":true}` || captured.Headers.Get("X-Webhook-Signature") != "secret" {
		t.Fatalf("unexpected captured request: %#v", captured)
	}
}

func TestHookHealthSummaryNamesLatestRequest(t *testing.T) {
	hook := newServer()
	request := httptest.NewRequest(http.MethodPost, "/hook/stripe", strings.NewReader(`{"ok":true}`))
	response := httptest.NewRecorder()
	hook.routes().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("capture status = %d", response.Code)
	}
	count, latest := hook.healthSummary()
	if count != 1 || latest != "POST /hook/stripe" {
		t.Fatalf("health summary = %d %q", count, latest)
	}
}

func TestCaptureRejectsOversizedBodiesAndCapsHistory(t *testing.T) {
	hook := newServer()
	handler := hook.routes()

	response := httptest.NewRecorder()
	oversized := bytes.NewReader(make([]byte, maxRequestBody+1))
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/hook", oversized))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d", response.Code)
	}
	if hook.count() != 0 {
		t.Fatal("oversized request was added to history")
	}

	for index := 0; index < maxHistoryItems+3; index++ {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/hook/"+string(rune('a'+index%26)), nil))
		if response.Code != http.StatusAccepted {
			t.Fatalf("capture %d status = %d", index, response.Code)
		}
	}
	if hook.count() != maxHistoryItems {
		t.Fatalf("history count = %d", hook.count())
	}
}

func TestReplayPreservesRequestAndRecordsResponse(t *testing.T) {
	var replayedMethod string
	var replayedHeader string
	var replayedBody string
	destination := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		replayedMethod = request.Method
		replayedHeader = request.Header.Get("X-Webhook-Signature")
		body, _ := io.ReadAll(request.Body)
		replayedBody = string(body)
		response.Header().Set("X-Destination", "test")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{"stored":true}`))
	}))
	defer destination.Close()

	hook := newServer()
	handler := hook.routes()
	capture := httptest.NewRequest(http.MethodPatch, "/hook/customer", strings.NewReader(`{"name":"Ada"}`))
	capture.Header.Set("Content-Type", "application/json")
	capture.Header.Set("X-Webhook-Signature", "signed-value")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, capture)
	id := response.Header().Get("X-Spare-Hook-Request-ID")

	payload, _ := json.Marshal(map[string]string{"targetUrl": destination.URL + "/receive"})
	response = httptest.NewRecorder()
	replay := httptest.NewRequest(http.MethodPost, "/api/requests/"+id+"/replay", bytes.NewReader(payload))
	replay.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, replay)
	if response.Code != http.StatusOK {
		t.Fatalf("replay status = %d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Replay replayAttempt `json:"replay"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Replay.Status != "completed" || result.Replay.StatusCode != http.StatusCreated ||
		result.Replay.ResponseBody != `{"stored":true}` {
		t.Fatalf("unexpected replay result: %#v", result.Replay)
	}
	if replayedMethod != http.MethodPatch || replayedHeader != "signed-value" || replayedBody != `{"name":"Ada"}` {
		t.Fatalf("replayed request = %s %q %q", replayedMethod, replayedHeader, replayedBody)
	}

	captured, ok := hook.get(id)
	if !ok || len(captured.Replays) != 1 || captured.Replays[0].StatusCode != http.StatusCreated {
		t.Fatalf("replay was not recorded: %#v", captured)
	}
}

func TestReplayRejectsUnsafeInputAndCrossOriginBrowserRequests(t *testing.T) {
	hook := newServer()
	handler := hook.routes()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/hook", nil))
	id := response.Header().Get("X-Spare-Hook-Request-ID")

	for name, target := range map[string]string{
		"relative":    "/internal",
		"credentials": "https://user:password@example.com/hook",
		"fragment":    "https://example.com/hook#secret",
		"scheme":      "file:///tmp/hook",
	} {
		t.Run(name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]string{"targetUrl": target})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/requests/"+id+"/replay", bytes.NewReader(payload))
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("target %q status = %d", target, response.Code)
			}
		})
	}

	payload, _ := json.Marshal(map[string]string{"targetUrl": "https://example.com/hook"})
	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/requests/"+id+"/replay", bytes.NewReader(payload))
	request.Host = "hook.local:7340"
	request.Header.Set("Origin", "https://attacker.example")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin replay status = %d", response.Code)
	}
}

func TestPageOnlyAllowsSpareAndLocalPreviewFraming(t *testing.T) {
	response := httptest.NewRecorder()
	newServer().routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("page status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "See every request") {
		t.Fatal("Hook page content is missing")
	}
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "frame-ancestors 'self' wails://wails") {
		t.Fatal("Hook page does not restrict framing to Spare")
	}
}

func TestReplayHistoryIsBounded(t *testing.T) {
	hook := newServer()
	captured := &capturedRequest{ID: "request", Replays: []replayAttempt{}}
	hook.add(captured)
	for index := 0; index < maxReplayItems+3; index++ {
		hook.addReplay("request", replayAttempt{ID: string(rune('a' + index))})
	}
	stored, ok := hook.get("request")
	if !ok {
		t.Fatal("captured request is missing")
	}
	if len(stored.Replays) != maxReplayItems {
		t.Fatalf("replay count = %d", len(stored.Replays))
	}
}
