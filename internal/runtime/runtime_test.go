package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"jproxy-go/internal/format"
	"jproxy-go/internal/store/sqlite"
)

type testLoader struct {
	snapshot sqlite.Snapshot
	err      error
	loads    int
}

func (l *testLoader) LoadFormatterSnapshot(context.Context) (sqlite.Snapshot, error) {
	l.loads++
	return l.snapshot, l.err
}

type testCache struct {
	clears  int
	deleted []string
}

func (c *testCache) Clear()            { c.clears++ }
func (c *testCache) Delete(key string) { c.deleted = append(c.deleted, key) }

func TestRegistryInvalidate_refreshesOnlyNamedRevision_andKeepsUnrelatedCaches(t *testing.T) {
	// Given
	loader := &testLoader{snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)
	before := provider.Snapshot()

	// When
	err := registry.Invalidate(context.Background(), RadarrRule, RadarrRule)

	// Then
	if err != nil || provider.Snapshot().RadarrRevision == before.RadarrRevision || provider.Snapshot().SonarrRevision != before.SonarrRevision || results.clears != 0 || offsets.clears != 0 {
		t.Fatalf("err=%v before=%+v after=%+v results=%d offsets=%d", err, before, provider.Snapshot(), results.clears, offsets.clears)
	}
}

func TestRegistryInvalidate_keepsLastKnownGood_andHasNoPartialEffects_whenRefreshFails(t *testing.T) {
	// Given
	const canary = "file:C:/private/jproxy.db?apikey=secret"
	loader := &testLoader{err: errors.New(canary)}
	provider := NewProvider(testSnapshot("old"), loader)
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)
	before := provider.Snapshot()

	// When
	err := registry.Invalidate(context.Background(), RadarrRule, IndexerResult)

	// Then
	after := provider.Snapshot()
	if !errors.Is(err, ErrSnapshotRefresh) || strings.Contains(err.Error(), canary) || after.RadarrRevision != before.RadarrRevision || after.SonarrRevision != before.SonarrRevision || after.RadarrSearchRevision != before.RadarrSearchRevision || after.SonarrSearchRevision != before.SonarrSearchRevision || results.clears != 0 || offsets.clears != 0 || len(markers.deleted) != 0 {
		t.Fatalf("err=%v snapshot=%+v results=%d offsets=%d", err, provider.Snapshot(), results.clears, offsets.clears)
	}
}

type canaryRefreshCause struct {
	values []string
}

func (e *canaryRefreshCause) Error() string { return strings.Join(e.values, " ") }

func (e *canaryRefreshCause) Format(state fmt.State, verb rune) {
	_, _ = fmt.Fprintf(state, "%%%c:%s", verb, e.Error())
}

func TestProviderRefresh_errorGraphDoesNotExposeRawCauseOrCanaries(t *testing.T) {
	// Given
	canaries := []string{
		`C:\private\jproxy.db`,
		`file:C:/private/jproxy.db?mode=rw&_pragma=busy_timeout(5000)`,
		`apikey=secret-api-key`,
		`token=secret-token`,
		`password=secret-password`,
		`regex=(?<secret>.*)`,
		`<rss><channel><item><title>secret</title></item></channel></rss>`,
	}
	loader := &testLoader{err: &canaryRefreshCause{values: canaries}}
	provider := NewProvider(testSnapshot("old"), loader)

	// When
	err := provider.Refresh(context.Background(), ScopeRadarrRules)

	// Then
	if !errors.Is(err, ErrSnapshotRefresh) {
		t.Fatalf("Refresh() error = %v, want ErrSnapshotRefresh", err)
	}
	var cause *canaryRefreshCause
	if errors.As(err, &cause) {
		t.Fatalf("raw refresh cause is reachable through errors.As: %#v", cause)
	}
	for _, text := range errorGraphTexts(err) {
		for _, canary := range canaries {
			if strings.Contains(text, canary) {
				t.Fatalf("error graph exposed canary %q in %q", canary, text)
			}
		}
	}
	t.Logf("task5_runtime_qa redacted_error=true canary_count=%d typed_category=%t", len(canaries), errors.Is(err, ErrSnapshotRefresh))
}

func errorGraphTexts(err error) []string {
	if err == nil {
		return nil
	}
	texts := []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		texts = append(texts, errorGraphTexts(wrapped.Unwrap())...)
	}
	if wrapped, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range wrapped.Unwrap() {
			texts = append(texts, errorGraphTexts(child)...)
		}
	}
	return texts
}

func TestProviderSnapshot_returnsDeepCopy(t *testing.T) {
	// Given
	provider := NewStaticProvider(testSnapshot("old"))
	first := provider.Snapshot()

	// When
	first.Radarr.Rules[0].Replacement = "mutated"
	first.Radarr.Rules[0].ValidStatus = new(int)

	// Then
	second := provider.Snapshot()
	if second.Radarr.Rules[0].Replacement != "old" || second.Radarr.Rules[0].ValidStatus != nil {
		t.Fatalf("published snapshot was mutated: %+v", second)
	}
	t.Logf("task5_runtime_qa deep_snapshot_copy=true pointer_copy=true")
}

func TestProvider_publishesQBittorrentConfigOnlyForSystemScope(t *testing.T) {
	// Given
	initial := testSnapshot("old")
	initial.QBittorrentURL = "http://old.example"
	initial.QBittorrentUsername = "old-user"
	initial.QBittorrentPassword = "old-password"
	loaded := testSnapshot("new")
	loaded.QBittorrentURL = "http://new.example"
	loaded.QBittorrentUsername = "new-user"
	loaded.QBittorrentPassword = "new-password"
	provider := NewProvider(initial, &testLoader{snapshot: loaded})
	before := provider.Snapshot()

	// When
	if err := provider.Refresh(context.Background(), ScopeRadarrRules); err != nil {
		t.Fatal(err)
	}
	afterUnrelated := provider.Snapshot()
	if err := provider.Refresh(context.Background(), ScopeSystemConfig); err != nil {
		t.Fatal(err)
	}
	afterSystem := provider.Snapshot()

	// Then
	if before.QBittorrentRevision == 0 || afterUnrelated.QBittorrentURL != initial.QBittorrentURL || afterUnrelated.QBittorrentRevision != before.QBittorrentRevision || afterSystem.QBittorrentURL != loaded.QBittorrentURL || afterSystem.QBittorrentUsername != loaded.QBittorrentUsername || afterSystem.QBittorrentPassword != loaded.QBittorrentPassword || afterSystem.QBittorrentRevision != before.QBittorrentRevision+1 {
		t.Fatalf("before=%+v unrelated=%+v system=%+v", before, afterUnrelated, afterSystem)
	}
}

func TestProvider_publishesTransmissionConfigOnlyForSystemScope(t *testing.T) {
	// Given
	initial := testSnapshot("old")
	initial.TransmissionURL = "http://old.example/transmission/rpc"
	initial.TransmissionUsername = "old-user"
	initial.TransmissionPassword = "old-password"
	loaded := testSnapshot("new")
	loaded.TransmissionURL = "http://new.example/transmission/rpc"
	loaded.TransmissionUsername = "new-user"
	loaded.TransmissionPassword = "new-password"
	provider := NewProvider(initial, &testLoader{snapshot: loaded})
	before := provider.Snapshot()

	// When
	if err := provider.Refresh(context.Background(), ScopeRadarrRules); err != nil {
		t.Fatal(err)
	}
	afterUnrelated := provider.Snapshot()
	if err := provider.Refresh(context.Background(), ScopeSystemConfig); err != nil {
		t.Fatal(err)
	}
	afterSystem := provider.Snapshot()

	// Then
	if before.TransmissionRevision == 0 || afterUnrelated.TransmissionURL != initial.TransmissionURL || afterUnrelated.TransmissionRevision != before.TransmissionRevision || afterSystem.TransmissionURL != loaded.TransmissionURL || afterSystem.TransmissionUsername != loaded.TransmissionUsername || afterSystem.TransmissionPassword != loaded.TransmissionPassword || afterSystem.TransmissionRevision != before.TransmissionRevision+1 || afterSystem.QBittorrentRevision != before.QBittorrentRevision+1 {
		t.Fatal("Transmission configuration or revision did not follow system-config publication semantics")
	}
}

func TestPublishPrepared_replacesQBittorrentConfigAndRevision(t *testing.T) {
	// Given
	initial := testSnapshot("old")
	initial.QBittorrentURL, initial.QBittorrentUsername, initial.QBittorrentPassword = "http://old", "old-user", "old-password"
	provider := NewStaticProvider(initial)
	before := provider.Snapshot()
	next := testSnapshot("new")
	next.QBittorrentURL, next.QBittorrentUsername, next.QBittorrentPassword = "http://new", "new-user", "new-password"

	// When
	published := PublishPrepared(provider, next)
	after := provider.Snapshot()

	// Then
	if !published || after.QBittorrentURL != next.QBittorrentURL || after.QBittorrentUsername != next.QBittorrentUsername || after.QBittorrentPassword != next.QBittorrentPassword || after.QBittorrentRevision != before.QBittorrentRevision+1 {
		t.Fatalf("published=%t before=%+v after=%+v", published, before, after)
	}
}

func TestPublishPrepared_replacesTransmissionConfigAndRevision(t *testing.T) {
	// Given
	initial := testSnapshot("old")
	initial.TransmissionURL, initial.TransmissionUsername, initial.TransmissionPassword = "http://old/transmission/rpc", "old-user", "old-password"
	provider := NewStaticProvider(initial)
	before := provider.Snapshot()
	next := testSnapshot("new")
	next.TransmissionURL, next.TransmissionUsername, next.TransmissionPassword = "http://new/transmission/rpc", "new-user", "new-password"

	// When
	published := PublishPrepared(provider, next)
	after := provider.Snapshot()

	// Then
	if !published || after.TransmissionURL != next.TransmissionURL || after.TransmissionUsername != next.TransmissionUsername || after.TransmissionPassword != next.TransmissionPassword || after.TransmissionRevision != before.TransmissionRevision+1 || after.QBittorrentRevision != before.QBittorrentRevision+1 {
		t.Fatal("prepared publication did not replace Transmission configuration and revisions")
	}
}

func TestRegistryInvalidate_advancesOnlyMatchingSearchRevision(t *testing.T) {
	// Given
	loader := &testLoader{snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	registry := NewRegistry(provider, &testCache{}, &testCache{}, &testCache{})
	before := provider.Snapshot()

	// When
	err := registry.Invalidate(context.Background(), SonarrSearchTitle)

	// Then
	after := provider.Snapshot()
	if err != nil || after.SonarrSearchRevision == before.SonarrSearchRevision || after.RadarrSearchRevision != before.RadarrSearchRevision {
		t.Fatalf("err=%v before=%+v after=%+v", err, before, after)
	}
	t.Logf("task5_runtime_qa sonarr_search_revision_changed=%t radarr_search_revision_unchanged=%t", after.SonarrSearchRevision != before.SonarrSearchRevision, after.RadarrSearchRevision == before.RadarrSearchRevision)
}

func TestRegistryInvalidate_advancesOnlyRadarrSearchRevision_whenRadarrTitlesChange(t *testing.T) {
	// Given
	loader := &testLoader{snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	registry := NewRegistry(provider, &testCache{}, &testCache{}, &testCache{})
	before := provider.Snapshot()

	// When
	err := registry.Invalidate(context.Background(), RadarrSearchTitle)

	// Then
	after := provider.Snapshot()
	if err != nil || after.RadarrSearchRevision == before.RadarrSearchRevision || after.SonarrSearchRevision != before.SonarrSearchRevision {
		t.Fatalf("err=%v before=%+v after=%+v", err, before, after)
	}
	t.Logf("task5_runtime_qa radarr_search_revision_changed=%t sonarr_search_revision_unchanged=%t", after.RadarrSearchRevision != before.RadarrSearchRevision, after.SonarrSearchRevision == before.SonarrSearchRevision)
}

func TestRegistryInvalidate_rejectsUnknownNames_beforeEffects(t *testing.T) {
	// Given
	provider := NewStaticProvider(testSnapshot("old"))
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)

	// When
	err := registry.Invalidate(context.Background(), IndexerResult, " ")

	// Then
	if !errors.Is(err, ErrUnknownCacheName) || results.clears != 0 {
		t.Fatalf("err=%v clears=%d", err, results.clears)
	}
}

func testSnapshot(title string) sqlite.Snapshot {
	return sqlite.Snapshot{Radarr: format.Config{Format: "{title}", Rules: []format.Rule{{Token: "title", Regex: ".*", Replacement: title}}}}
}
