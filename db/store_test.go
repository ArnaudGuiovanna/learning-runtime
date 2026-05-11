package db

import (
	"errors"
	"testing"
	"time"
)

func TestConsumeAuthCode_WrongClientID(t *testing.T) {
	store := setupTestDB(t)

	if err := store.CreateOAuthClient("client-A", "Client A", `["https://a.example/cb"]`); err != nil {
		t.Fatalf("create client A: %v", err)
	}
	if err := store.CreateOAuthClient("client-B", "Client B", `["https://b.example/cb"]`); err != nil {
		t.Fatalf("create client B: %v", err)
	}

	expires := time.Now().Add(5 * time.Minute)
	if err := store.CreateAuthCode("code-1", "L1", "chal", "client-A", expires); err != nil {
		t.Fatalf("create code: %v", err)
	}

	if _, err := store.ConsumeAuthCode("code-1", "client-B"); err == nil {
		t.Fatal("expected error when consuming with wrong client_id")
	}

	// Code must still be present after failed consume attempt
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM oauth_codes WHERE code = ?`, "code-1").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected code still present, got count=%d", count)
	}

	ac, err := store.ConsumeAuthCode("code-1", "client-A")
	if err != nil {
		t.Fatalf("consume with correct client: %v", err)
	}
	if ac.ClientID != "client-A" || ac.LearnerID != "L1" {
		t.Fatalf("unexpected ac: %+v", ac)
	}

	if err := store.db.QueryRow(`SELECT COUNT(*) FROM oauth_codes WHERE code = ?`, "code-1").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected code consumed, got count=%d", count)
	}
}

func TestGetOAuthClient(t *testing.T) {
	store := setupTestDB(t)
	if err := store.CreateOAuthClient("c1", "n1", `["https://x.example/cb"]`); err != nil {
		t.Fatalf("create: %v", err)
	}
	c, err := store.GetOAuthClient("c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if c.ClientID != "c1" || c.ClientName != "n1" || c.RedirectURIs != `["https://x.example/cb"]` {
		t.Fatalf("unexpected client: %+v", c)
	}
	if _, err := store.GetOAuthClient("missing"); err == nil {
		t.Fatal("expected error for missing client")
	}
}

func TestCountOAuthClients(t *testing.T) {
	store := setupTestDB(t)
	if err := store.CreateOAuthClient("c1", "n1", `["https://x.example/cb"]`); err != nil {
		t.Fatalf("create c1: %v", err)
	}
	if err := store.CreateOAuthClient("c2", "n2", `["https://y.example/cb"]`); err != nil {
		t.Fatalf("create c2: %v", err)
	}

	got, err := store.CountOAuthClients()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
}

func TestCreateOAuthClientWithSecretCappedRejectsAtLimit(t *testing.T) {
	store := setupTestDB(t)
	if err := store.CreateOAuthClientWithSecretCapped("c1", "n1", `["https://x.example/cb"]`, "", 1); err != nil {
		t.Fatalf("create c1: %v", err)
	}

	err := store.CreateOAuthClientWithSecretCapped("c2", "n2", `["https://y.example/cb"]`, "", 1)
	if !errors.Is(err, ErrOAuthClientLimitReached) {
		t.Fatalf("err = %v, want ErrOAuthClientLimitReached", err)
	}
	got, err := store.CountOAuthClients()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
}
