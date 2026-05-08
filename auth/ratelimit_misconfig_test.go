// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna/tutor-mcp
// SPDX-License-Identifier: MIT

package auth

import "testing"

// Reproducer for issue #37: per-IP rate limiter bucket key collapses to "the
// proxy" when TRUSTED_PROXY_CIDRS is unset. Two sub-conditions are guarded:
//   1. parseTrustedProxiesCIDRs rejects 0.0.0.0/0 and ::/0 (catch-alls).
//   2. shouldWarnRateLimiterMisconfig fires when BASE_URL is public and
//      TRUSTED_PROXY_CIDRS is empty.

func TestParseTrustedProxiesCIDRs_RejectsCatchAll(t *testing.T) {
	cases := []string{"0.0.0.0/0", "::/0", "10.0.0.0/8, 0.0.0.0/0", "::/0,192.168.0.0/16"}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			got := parseTrustedProxiesCIDRs(raw)
			for _, cidr := range got {
				if ones, _ := cidr.Mask.Size(); ones == 0 {
					t.Errorf("catch-all CIDR slipped through: %v", cidr)
				}
			}
		})
	}
}

func TestParseTrustedProxiesCIDRs_AcceptsValidCIDRs(t *testing.T) {
	got := parseTrustedProxiesCIDRs("10.0.0.0/8, 127.0.0.1/32, fc00::/7")
	if len(got) != 3 {
		t.Fatalf("expected 3 valid CIDRs, got %d: %v", len(got), got)
	}
}

func TestShouldWarnRateLimiterMisconfig(t *testing.T) {
	cases := []struct {
		name           string
		baseURL        string
		trustedProxies string
		want           bool
	}{
		{"public https no proxies — warn", "https://mcp.example.com", "", true},
		{"public https with proxies — silent", "https://mcp.example.com", "10.0.0.0/8", false},
		{"http (local dev) — silent", "http://localhost:8080", "", false},
		{"https localhost — silent", "https://localhost", "", false},
		{"https loopback IPv4 — silent", "https://127.0.0.1", "", false},
		{"https loopback IPv6 — silent", "https://[::1]", "", false},
		{"unparseable URL — silent", "::not a url::", "", false},
		{"empty URL — silent", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldWarnRateLimiterMisconfig(c.baseURL, c.trustedProxies); got != c.want {
				t.Errorf("shouldWarn(%q, %q) = %v, want %v", c.baseURL, c.trustedProxies, got, c.want)
			}
		})
	}
}
