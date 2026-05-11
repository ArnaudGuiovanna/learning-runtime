package engine

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tutor-mcp/db"
)

func TestIsSafeWebhookURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"https discord.com", "https://discord.com/api/webhooks/123/abc", true},
		{"https ptb.discord.com", "https://ptb.discord.com/api/webhooks/123/abc", true},
		{"https discordapp.com", "https://discordapp.com/api/webhooks/123/abc", true},
		{"http discord.com rejected", "http://discord.com/api/webhooks/123/abc", false},
		{"https evil.com rejected", "https://evil.com/api/webhooks/123/abc", false},
		{"https IMDS rejected", "https://169.254.169.254/latest/meta-data/", false},
		{"empty rejected", "", false},
		{"suffix spoof rejected", "https://evildiscord.com/x", false},
		{"double dot rejected", "https://discord..com/x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := db.IsSafeWebhookURL(tc.url); got != tc.want {
				t.Errorf("IsSafeWebhookURL(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestSendOLM_DispatchesFallbackWhenQueueEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got := string(body)
		if !strings.Contains(got, "Current focus") &&
			!strings.Contains(got, "Next milestone") &&
			!strings.Contains(got, "needs attention now") {
			t.Errorf("expected an OLM body, got: %s", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	prev := safeWebhookURL
	safeWebhookURL = func(_ string) bool { return true }
	defer func() { safeWebhookURL = prev }()

	store, raw := newOLMTestStore(t)
	if _, err := raw.Exec(
		`INSERT INTO learners (id, email, password_hash, objective, webhook_url, last_active, created_at) VALUES (?,?,?,?,?,?,?)`,
		"L1", "l1@t.com", "h", "obj", srv.URL, time.Now().UTC(), time.Now().UTC(),
	); err != nil {
		t.Fatal(err)
	}
	seedDomain(t, raw, "L1", "math",
		[]string{"a", "b"},
		map[string][]string{"b": {"a"}},
		false,
	)
	seedConceptState(t, store, "L1", "a", 0.90, "review")

	sched := schedulerForTest(store)
	sched.sendOLM()

	sent, _ := store.WasAlertSentToday("L1", alertKindOLM)
	if !sent {
		t.Errorf("OLM dispatch should mark sent today")
	}
}

func TestSendOLM_SkipsWhenNothingActionable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("scheduler should NOT post when nothing is actionable")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	prev := safeWebhookURL
	safeWebhookURL = func(_ string) bool { return true }
	defer func() { safeWebhookURL = prev }()

	store, raw := newOLMTestStore(t)
	if _, err := raw.Exec(
		`INSERT INTO learners (id, email, password_hash, objective, webhook_url, last_active, created_at) VALUES (?,?,?,?,?,?,?)`,
		"L1", "l1@t.com", "h", "obj", srv.URL, time.Now().UTC(), time.Now().UTC(),
	); err != nil {
		t.Fatal(err)
	}
	seedDomain(t, raw, "L1", "math", []string{"a"}, nil, false)
	seedConceptState(t, store, "L1", "a", 0.90, "review")

	sched := schedulerForTest(store)
	sched.sendOLM()

	sent, _ := store.WasAlertSentToday("L1", alertKindOLM)
	if sent {
		t.Errorf("nothing actionable should NOT mark OLM as sent")
	}
}
