// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package apihttp

import (
	"sync"
	"time"
)

// contentStore is an in-memory rendezvous keyed by request_id.
// Producers (the submit_exercise_content tool) call Put(id, text);
// consumers (the /api/v1/exercise_content polling endpoint) call Get(id).
// Entries auto-expire after 5 minutes to bound memory.
type contentEntry struct {
	Text      string
	CreatedAt time.Time
}

var (
	contentMu    sync.Mutex
	contentStore = map[string]contentEntry{}
)

func PutContent(id, text string) {
	contentMu.Lock()
	defer contentMu.Unlock()
	contentStore[id] = contentEntry{Text: text, CreatedAt: time.Now()}
	// Opportunistic GC: drop entries older than 5 min.
	for k, v := range contentStore {
		if time.Since(v.CreatedAt) > 5*time.Minute {
			delete(contentStore, k)
		}
	}
}

func GetContent(id string) (string, bool) {
	contentMu.Lock()
	defer contentMu.Unlock()
	e, ok := contentStore[id]
	return e.Text, ok
}
