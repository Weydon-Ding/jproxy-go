package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestRuleRepository_exportsDeterministicallyAndImportsAtomically_whenRequested(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := store.Repositories().SonarrRules
	stamp := stringPointer("2026-08-05T12:00:00Z")
	rows := []SonarrRule{{ID: "b", Token: "z", Regex: "x", Example: "x", ValidStatus: Valid, UpdateTime: stamp}, {ID: "a", Token: "a", Regex: "x", Example: "x", ValidStatus: Valid, UpdateTime: stamp}}
	if err := repo.Import(ctx, SonarrRuleBatch{Rows: rows}); err != nil {
		t.Fatal(err)
	}
	all, err := repo.Export(ctx, RuleIDs{})
	if err != nil || len(all) != 2 || all[0].ID != "b" || all[1].ID != "a" {
		t.Fatalf("all=%+v err=%v", all, err)
	}
	selected, err := repo.Export(ctx, RuleIDs{IDs: []RuleID{"a", "b"}})
	if err != nil || len(selected) != 2 || selected[0].ID != "b" {
		t.Fatalf("selected=%+v err=%v", selected, err)
	}
	tokens, err := repo.Tokens(ctx)
	if err != nil || len(tokens) != 2 || tokens[0] != "a" || tokens[1] != "z" {
		t.Fatalf("tokens=%v err=%v", tokens, err)
	}
	err = repo.Import(ctx, SonarrRuleBatch{Rows: []SonarrRule{{ID: "c", Token: "ok", Regex: "x", Example: "x", ValidStatus: Valid}, {ID: "d", Token: "bad", Regex: "x", Example: "x", ValidStatus: ValidStatus(2)}}})
	if !errors.Is(err, ErrInvalidValidStatus) {
		t.Fatalf("err=%v", err)
	}
	after, err := repo.Export(ctx, RuleIDs{})
	if err != nil || len(after) != 2 {
		t.Fatalf("after=%+v err=%v", after, err)
	}
}
