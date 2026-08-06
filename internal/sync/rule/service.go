package rulesync

import (
	"context"

	"jproxy-go/internal/store/sqlite"
)

type transactionStore interface {
	InTransaction(context.Context, func(sqlite.DatasetTransaction) error) error
}

type ServiceDependencies struct {
	Client  Client
	Store   transactionStore
	Authors string
}

type Service struct{ dependencies ServiceDependencies }

func NewService(dependencies ServiceDependencies) Service { return Service{dependencies: dependencies} }

func (s Service) SyncSonarr(ctx context.Context) (Report, error) { return s.sync(ctx, Sonarr) }
func (s Service) SyncRadarr(ctx context.Context) (Report, error) { return s.sync(ctx, Radarr) }

func (s Service) sync(ctx context.Context, domain Domain) (Report, error) {
	authors, err := normalizeAuthors(s.dependencies.Authors)
	if err != nil {
		return Report{}, err
	}
	if len(authors) == 1 && authors[0] == allAuthors {
		authors, err = s.dependencies.Client.Authors(ctx)
		if err != nil {
			return Report{}, err
		}
	}
	report := Report{Domain: domain, authors: make([]AuthorReport, 0, len(authors))}
	for _, author := range authors {
		report.authors = append(report.authors, s.syncAuthor(ctx, domain, author))
	}
	return report, nil
}

func (s Service) syncAuthor(ctx context.Context, domain Domain, author string) AuthorReport {
	rules, err := s.dependencies.Client.rules(ctx, domain, author)
	if err != nil {
		return AuthorReport{Author: author, Failure: FailureRemote}
	}
	if domain == Sonarr {
		return s.syncSonarrAuthor(ctx, author, rules)
	}
	return s.syncRadarrAuthor(ctx, author, rules)
}

func (s Service) syncSonarrAuthor(ctx context.Context, author string, rules []remoteRule) AuthorReport {
	inputs := make([]sqlite.SonarrRuleInput, len(rules))
	for index, rule := range rules {
		input, err := rule.sonarr(author)
		if err != nil {
			return AuthorReport{Author: author, Failure: FailureInvalidPayload}
		}
		inputs[index] = input
	}
	if err := s.dependencies.Store.InTransaction(ctx, func(tx sqlite.DatasetTransaction) error { return tx.UpsertRemoteSonarrRules(ctx, inputs) }); err != nil {
		return AuthorReport{Author: author, Failure: FailureTransaction}
	}
	return AuthorReport{Author: author, Committed: len(inputs)}
}

func (s Service) syncRadarrAuthor(ctx context.Context, author string, rules []remoteRule) AuthorReport {
	inputs := make([]sqlite.RadarrRuleInput, len(rules))
	for index, rule := range rules {
		input, err := rule.radarr(author)
		if err != nil {
			return AuthorReport{Author: author, Failure: FailureInvalidPayload}
		}
		inputs[index] = input
	}
	if err := s.dependencies.Store.InTransaction(ctx, func(tx sqlite.DatasetTransaction) error { return tx.UpsertRemoteRadarrRules(ctx, inputs) }); err != nil {
		return AuthorReport{Author: author, Failure: FailureTransaction}
	}
	return AuthorReport{Author: author, Committed: len(inputs)}
}
