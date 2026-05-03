// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package assets

import "testing"

func TestEmbeddedCockpitHTML_Present(t *testing.T) {
	data, err := FS.ReadFile("cockpit.html")
	if err != nil {
		t.Fatalf("read cockpit.html: %v", err)
	}
	if len(data) < 50 {
		t.Errorf("cockpit.html too small: %d bytes", len(data))
	}
}
