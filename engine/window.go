// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

// DefaultRecentInteractionsWindow is the default number of most-recent
// learner interactions fetched from the store and fed into downstream
// engine consumers (alerts, OLM focus, dashboard activity).
//
// Rationale for 20: balances responsiveness vs. statistical noise — it
// covers roughly the last ~5 sessions of activity at typical pacing
// (~4 items/session), which is enough history for ComputeAlerts to
// detect FRUSTRATION / FORGETTING patterns while staying small enough
// to avoid bleed-in from sessions that no longer reflect the learner's
// current state.
//
// Per-domain tunability is intentionally out of scope here; future work
// can wire this into LearnerProfile or DomainConfig as needed.
const DefaultRecentInteractionsWindow = 20
