package rulesync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"jproxy-go/internal/store/sqlite"
)

func TestService_syncsExplicitAuthorsInOrderAndReportsCommittedDomain(t *testing.T) {
	// Given
	server := ruleServer(t, map[string]string{
		"/sonarr@alice.json": validRule("alice", "alice-rule"),
		"/sonarr@bob.json":   validRule("bob", "bob-rule"),
	})
	defer server.Close()
	store := openRuleStore(t)
	defer store.Close()
	service := newRuleService(t, store, server.URL, " bob, alice ")

	// When
	report, err := service.SyncSonarr(context.Background())

	// Then
	authors := report.Authors()
	if err != nil || report.Domain != Sonarr || !report.Succeeded() || authors[0].Author != "bob" || authors[1].Author != "alice" || authors[0].Committed != 1 || authors[1].Committed != 1 || sonarrRuleCount(t, store) != 2 {
		t.Fatalf("report=%+v err=%v count=%d", report, err, sonarrRuleCount(t, store))
	}
}

func TestService_resolvesALLAndContinuesAfterPerAuthorRollback(t *testing.T) {
	// Given
	server := ruleServer(t, map[string]string{
		"/author.json":       `["good", "bad", "later"]`,
		"/radarr@good.json":  validRule("good", "good-rule"),
		"/radarr@bad.json":   `[{"id":"bad","token":"title","priority":1,"regex":"[","replacement":"x","offset":0,"example":"x","remark":"remark","author":"bad","validStatus":1}]`,
		"/radarr@later.json": validRule("later", "later-rule"),
	})
	defer server.Close()
	store := openRuleStore(t)
	defer store.Close()
	service := newRuleService(t, store, server.URL, "ALL")

	// When
	report, err := service.SyncRadarr(context.Background())

	// Then
	authors := report.Authors()
	if err != nil || report.Succeeded() || len(authors) != 3 || authors[0].Committed != 1 || authors[1].Failure != FailureInvalidPayload || authors[2].Committed != 1 || radarrRuleCount(t, store) != 2 {
		t.Fatalf("report=%+v err=%v count=%d", report, err, radarrRuleCount(t, store))
	}
}

func TestService_rejectsInvalidStatusTokenAndPayloadAuthorWithoutWrites(t *testing.T) {
	for name, payload := range map[string]string{
		"status": `[{"id":"a","token":"title","priority":1,"regex":"x","replacement":"","offset":0,"example":"x","remark":"remark","author":"alice","validStatus":2}]`,
		"token":  `[{"id":"a","token":"unknown","priority":1,"regex":"x","replacement":"","offset":0,"example":"x","remark":"remark","author":"alice","validStatus":1}]`,
		"author": `[{"id":"a","token":"title","priority":1,"regex":"x","replacement":"","offset":0,"example":"x","remark":"remark","author":"other","validStatus":1}]`,
	} {
		t.Run(name, func(t *testing.T) {
			// Given
			server := ruleServer(t, map[string]string{"/sonarr@alice.json": payload})
			defer server.Close()
			store := openRuleStore(t)
			defer store.Close()
			service := newRuleService(t, store, server.URL, "alice")

			// When
			report, err := service.SyncSonarr(context.Background())

			// Then
			if err != nil || report.Succeeded() || report.Authors()[0].Failure != FailureInvalidPayload || sonarrRuleCount(t, store) != 0 {
				t.Fatalf("report=%+v err=%v count=%d", report, err, sonarrRuleCount(t, store))
			}
		})
	}
}

func TestService_preservesExistingStatusWhenRemoteRuleIsValid(t *testing.T) {
	// Given
	server := ruleServer(t, map[string]string{"/sonarr@alice.json": validRule("alice", "rule")})
	defer server.Close()
	store := openRuleStore(t)
	defer store.Close()
	if err := store.Repositories().SonarrRules.Upsert(context.Background(), sqlite.SonarrRule{ID: "rule", Token: "title", Regex: "x", Example: "x", ValidStatus: sqlite.Invalid}); err != nil {
		t.Fatalf("SonarrRules.Upsert() error = %v", err)
	}
	service := newRuleService(t, store, server.URL, "alice")

	// When
	report, err := service.SyncSonarr(context.Background())

	// Then
	row, getErr := store.Repositories().SonarrRules.Get(context.Background(), "rule")
	if err != nil || getErr != nil || !report.Succeeded() || row.ValidStatus != sqlite.Invalid || row.Author == nil || *row.Author != "alice" {
		t.Fatalf("report=%+v syncErr=%v row=%+v getErr=%v", report, err, row, getErr)
	}
}

func TestService_reportsTransactionFailureAndContinuesWithLaterAuthor(t *testing.T) {
	// Given
	server := ruleServer(t, map[string]string{
		"/sonarr@first.json": validRule("first", "first-rule"),
		"/sonarr@later.json": validRule("later", "later-rule"),
	})
	defer server.Close()
	store := openRuleStore(t)
	defer store.Close()
	client, err := NewClient(ClientOptions{PrimaryURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	service := NewService(ServiceDependencies{Client: client, Store: &failingFirstTransactionStore{store: store}, Authors: "first,later"})

	// When
	report, syncErr := service.SyncSonarr(context.Background())

	// Then
	authors := report.Authors()
	if syncErr != nil || len(authors) != 2 || authors[0].Failure != FailureTransaction || authors[0].Committed != 0 || authors[1].Committed != 1 || sonarrRuleCount(t, store) != 1 {
		t.Fatalf("report=%+v err=%v count=%d", report, syncErr, sonarrRuleCount(t, store))
	}
}

func newRuleService(t *testing.T, store *sqlite.Store, primaryURL, authors string) Service {
	t.Helper()
	client, err := NewClient(ClientOptions{PrimaryURL: primaryURL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return NewService(ServiceDependencies{Client: client, Store: store, Authors: authors})
}

func openRuleStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return store
}

func ruleServer(t *testing.T, payloads map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		payload, ok := payloads[request.URL.Path]
		if !ok {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = writer.Write([]byte(payload))
	}))
}

func validRule(author, id string) string {
	return `[{"id":"` + id + `","token":"title","priority":1,"regex":"^x$","replacement":"y","offset":0,"example":"x","remark":"remark","author":"` + author + `","validStatus":1}]`
}

func sonarrRuleCount(t *testing.T, store *sqlite.Store) int {
	t.Helper()
	page, err := store.Repositories().SonarrRules.Page(context.Background(), sqlite.RuleFilter{Page: sqlite.PageInput{Current: 1, Size: 200}})
	if err != nil {
		t.Fatalf("SonarrRules.Page() error = %v", err)
	}
	return int(page.Total)
}

func radarrRuleCount(t *testing.T, store *sqlite.Store) int {
	t.Helper()
	page, err := store.Repositories().RadarrRules.Page(context.Background(), sqlite.RuleFilter{Page: sqlite.PageInput{Current: 1, Size: 200}})
	if err != nil {
		t.Fatalf("RadarrRules.Page() error = %v", err)
	}
	return int(page.Total)
}

type failingFirstTransactionStore struct {
	store  *sqlite.Store
	failed bool
}

func (s *failingFirstTransactionStore) InTransaction(ctx context.Context, operation func(sqlite.DatasetTransaction) error) error {
	if !s.failed {
		s.failed = true
		return s.store.InTransaction(ctx, func(transaction sqlite.DatasetTransaction) error {
			if err := operation(transaction); err != nil {
				return err
			}
			return errors.New("transaction canary")
		})
	}
	return s.store.InTransaction(ctx, operation)
}
