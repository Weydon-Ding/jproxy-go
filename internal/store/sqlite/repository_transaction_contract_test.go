package sqlite_test

import (
	"context"

	"jproxy-go/internal/store/sqlite"
)

type transactionFake struct{}

func (transactionFake) ReplaceRadarrTitles(context.Context, sqlite.RadarrTitleBatch) error {
	return nil
}
func (transactionFake) UpsertRadarrTitles(context.Context, sqlite.RadarrTitleBatch) error { return nil }
func (transactionFake) DeleteRadarrTitles(context.Context, sqlite.RadarrTitleIDs) error   { return nil }
func (transactionFake) UpsertSonarrRules(context.Context, sqlite.SonarrRuleBatch) error   { return nil }
func (transactionFake) UpsertRemoteSonarrRules(context.Context, []sqlite.SonarrRuleInput) error {
	return nil
}
func (transactionFake) DeleteSonarrRules(context.Context, sqlite.RuleIDs) error { return nil }
func (transactionFake) SwitchSonarrRuleStatus(context.Context, sqlite.RuleIDs, sqlite.ValidStatus) error {
	return nil
}
func (transactionFake) ReplaceSonarrRules(context.Context, sqlite.SonarrRuleBatch) error { return nil }
func (transactionFake) UpsertRadarrRules(context.Context, sqlite.RadarrRuleBatch) error  { return nil }
func (transactionFake) UpsertRemoteRadarrRules(context.Context, []sqlite.RadarrRuleInput) error {
	return nil
}
func (transactionFake) DeleteRadarrRules(context.Context, sqlite.RuleIDs) error { return nil }
func (transactionFake) SwitchRadarrRuleStatus(context.Context, sqlite.RuleIDs, sqlite.ValidStatus) error {
	return nil
}
func (transactionFake) ReplaceRadarrRules(context.Context, sqlite.RadarrRuleBatch) error  { return nil }
func (transactionFake) UpsertSonarrTitles(context.Context, sqlite.SonarrTitleBatch) error { return nil }
func (transactionFake) DeleteSonarrTitles(context.Context, sqlite.SonarrTitleIDs) error   { return nil }
func (transactionFake) ReplaceSonarrTitles(context.Context, sqlite.SonarrTitleBatch) error {
	return nil
}
func (transactionFake) UpsertTMDBTitles(context.Context, sqlite.TMDBTitleBatch) error  { return nil }
func (transactionFake) DeleteTMDBTitles(context.Context, sqlite.TMDBTitleIDs) error    { return nil }
func (transactionFake) ReplaceTMDBTitles(context.Context, sqlite.TMDBTitleBatch) error { return nil }
func (transactionFake) UpsertSonarrExamples(context.Context, sqlite.SonarrExampleBatch) error {
	return nil
}
func (transactionFake) DeleteSonarrExamples(context.Context, sqlite.ExampleIDs) error { return nil }
func (transactionFake) ReplaceSonarrExamples(context.Context, sqlite.SonarrExampleBatch) error {
	return nil
}
func (transactionFake) UpsertRadarrExamples(context.Context, sqlite.RadarrExampleBatch) error {
	return nil
}
func (transactionFake) DeleteRadarrExamples(context.Context, sqlite.ExampleIDs) error { return nil }
func (transactionFake) ReplaceRadarrExamples(context.Context, sqlite.RadarrExampleBatch) error {
	return nil
}
func (transactionFake) ImportSonarrRules(context.Context, sqlite.SonarrRuleBatch) error { return nil }
func (transactionFake) ImportRadarrRules(context.Context, sqlite.RadarrRuleBatch) error { return nil }

var _ sqlite.DatasetTransaction = transactionFake{}
