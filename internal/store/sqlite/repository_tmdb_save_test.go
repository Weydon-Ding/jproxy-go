package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestTMDBTitleSave_upsertsSuppliedID_whenRowExists(t *testing.T) {
	store := openTMDBSaveStore(t)
	repo := store.Repositories().TMDBTitles
	input := TMDBTitleSaveInput{Title: TMDBTitle{ID: 8, TVDBID: 7, Language: "en", Title: "first", ValidStatus: Valid}, SuppliedID: true}
	if _, err := repo.Save(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	result, err := repo.Save(context.Background(), TMDBTitleSaveInput{Title: TMDBTitle{ID: 8, TVDBID: 7, Language: "en", Title: "updated", ValidStatus: Valid}, SuppliedID: true})
	if err != nil || result.ID != 8 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	row, err := repo.Get(context.Background(), 8)
	if err != nil || row.Title != "updated" {
		t.Fatalf("row=%+v err=%v", row, err)
	}
}

func TestTMDBTitleSave_generatesIDAndReusesFirstTMDBID_whenMissing(t *testing.T) {
	store := openTMDBSaveStore(t)
	repo := store.Repositories().TMDBTitles
	first, second := int64(101), int64(202)
	for _, row := range []TMDBTitle{{ID: 4, TVDBID: 7, TMDBID: &first, Language: "en", Title: "first", ValidStatus: Valid}, {ID: 9, TVDBID: 7, TMDBID: &second, Language: "zh", Title: "second", ValidStatus: Valid}} {
		if err := repo.Upsert(context.Background(), row); err != nil {
			t.Fatal(err)
		}
	}
	result, err := repo.Save(context.Background(), TMDBTitleSaveInput{Title: TMDBTitle{TVDBID: 7, Language: "ja", Title: "generated", ValidStatus: Valid}})
	if err != nil || result.ID == 0 || !result.Generated {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	row, err := repo.Get(context.Background(), result.ID)
	if err != nil || row.TMDBID == nil || *row.TMDBID != first {
		t.Fatalf("row=%+v err=%v", row, err)
	}
}

func TestTMDBTitleSave_keepsNilTMDBID_whenNoReusableRow(t *testing.T) {
	store := openTMDBSaveStore(t)
	result, err := store.Repositories().TMDBTitles.Save(context.Background(), TMDBTitleSaveInput{Title: TMDBTitle{TVDBID: 44, Language: "en", Title: "new", ValidStatus: Valid}})
	if err != nil {
		t.Fatal(err)
	}
	row, err := store.Repositories().TMDBTitles.Get(context.Background(), result.ID)
	if err != nil || row.TMDBID != nil {
		t.Fatalf("row=%+v err=%v", row, err)
	}
}

func TestTMDBTitleSave_rejectsInvalidAndCancelledInputs_withoutWrites(t *testing.T) {
	store := openTMDBSaveStore(t)
	repo := store.Repositories().TMDBTitles
	for _, input := range []TMDBTitleSaveInput{{Title: TMDBTitle{TVDBID: 1, Language: "en", Title: "bad", ValidStatus: ValidStatus(2)}}, {Title: TMDBTitle{TVDBID: 2147483648, Language: "en", Title: "bad", ValidStatus: Valid}}} {
		if _, err := repo.Save(context.Background(), input); err == nil {
			t.Fatal("want validation error")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.Save(ctx, TMDBTitleSaveInput{Title: TMDBTitle{TVDBID: 1, Language: "en", Title: "cancel", ValidStatus: Valid}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	page, err := repo.Page(context.Background(), TMDBTitleFilter{})
	if err != nil || page.Total != 0 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestTMDBTitleSave_serializesConcurrentGeneratedRows_whenReusingTMDBID(t *testing.T) {
	store := openTMDBSaveStore(t)
	repo := store.Repositories().TMDBTitles
	reusable := int64(77)
	if err := repo.Upsert(context.Background(), TMDBTitle{ID: 1, TVDBID: 7, TMDBID: &reusable, Language: "en", Title: "source", ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan TMDBTitleSaveResult, 8)
	failures := make(chan error, 8)
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := repo.Save(context.Background(), TMDBTitleSaveInput{Title: TMDBTitle{TVDBID: 7, Language: "x", Title: "row", ValidStatus: Valid}})
			if err != nil {
				failures <- err
				return
			}
			results <- result
		}()
	}
	group.Wait()
	close(results)
	close(failures)
	if err := <-failures; err != nil {
		t.Fatal(err)
	}
	seen := map[TMDBTitleID]bool{}
	for result := range results {
		if seen[result.ID] {
			t.Fatalf("duplicate id %d", result.ID)
		}
		seen[result.ID] = true
		row, err := repo.Get(context.Background(), result.ID)
		if err != nil || row.TMDBID == nil || *row.TMDBID != reusable {
			t.Fatalf("row=%+v err=%v", row, err)
		}
	}
	if len(seen) != 8 {
		t.Fatalf("saved=%d", len(seen))
	}
}

func openTMDBSaveStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "tmdb-save.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
