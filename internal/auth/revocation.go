package auth

import "sync"

type revocations struct {
	mu      sync.Mutex
	entries map[string]int64
	limit   int
}

func newRevocations(limit int) *revocations {
	return &revocations{entries: make(map[string]int64), limit: limit}
}

func (r *revocations) add(jti string, exp, now int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clean(now)
	if len(r.entries) >= r.limit {
		var earliestJTI string
		var earliestExpiry int64
		for existingJTI, existingExpiry := range r.entries {
			if earliestJTI == "" || existingExpiry < earliestExpiry {
				earliestJTI = existingJTI
				earliestExpiry = existingExpiry
			}
		}
		delete(r.entries, earliestJTI)
	}
	r.entries[jti] = exp
}

func (r *revocations) contains(jti string, now int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clean(now)
	_, found := r.entries[jti]
	return found
}

func (r *revocations) clean(now int64) {
	for jti, exp := range r.entries {
		if exp <= now {
			delete(r.entries, jti)
		}
	}
}
