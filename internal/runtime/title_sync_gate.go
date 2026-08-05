package runtime

import (
	"errors"
	"fmt"
	"sync"
)

var ErrTitleSyncTooFrequent = errors.New("title sync too frequent")

type TitleSyncAttempt struct {
	marker     string
	generation uint64
	id         uint64
}

type titleSyncState struct {
	inFlight        bool
	retainedSuccess bool
	generation      uint64
	attemptID       uint64
}

type titleSyncGate struct {
	mu     sync.Mutex
	states map[string]titleSyncState
	nextID uint64
}

func newTitleSyncGate() titleSyncGate {
	return titleSyncGate{states: map[string]titleSyncState{
		SonarrTitleSyncInterval: {},
		TMDBTitleSyncInterval:   {},
		RadarrTitleSyncInterval: {},
	}}
}

func (g *titleSyncGate) begin(marker string) (TitleSyncAttempt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	state, ok := g.states[marker]
	if !ok {
		return TitleSyncAttempt{}, fmt.Errorf("%q: %w", marker, ErrUnknownCacheName)
	}
	if state.inFlight || state.retainedSuccess {
		return TitleSyncAttempt{}, ErrTitleSyncTooFrequent
	}
	g.nextID++
	state.inFlight = true
	state.attemptID = g.nextID
	g.states[marker] = state
	return TitleSyncAttempt{marker: marker, generation: state.generation, id: state.attemptID}, nil
}

func (g *titleSyncGate) finish(attempt TitleSyncAttempt, succeeded bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	state, ok := g.states[attempt.marker]
	if !ok || state.attemptID != attempt.id || !state.inFlight {
		return
	}
	state.inFlight = false
	if state.generation == attempt.generation {
		state.retainedSuccess = succeeded
	}
	g.states[attempt.marker] = state
}

func (g *titleSyncGate) clear(marker string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.states[marker]
	state.generation++
	state.retainedSuccess = false
	g.states[marker] = state
}

func (g *titleSyncGate) clearAll() {
	for _, marker := range titleSyncMarkers {
		g.clear(marker)
	}
}

var titleSyncMarkers = []string{SonarrTitleSyncInterval, TMDBTitleSyncInterval, RadarrTitleSyncInterval}
