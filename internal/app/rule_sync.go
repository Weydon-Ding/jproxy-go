package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"jproxy-go/internal/api/rule"
	"jproxy-go/internal/api/system"
	"jproxy-go/internal/config"
	"jproxy-go/internal/store/sqlite"
	rulesync "jproxy-go/internal/sync/rule"
)

var errPartialRuleSync = errors.New("rule sync incomplete")

type ruleSyncConfigSource struct{ store managementStore }

type ruleTransactionStore interface {
	InTransaction(context.Context, func(sqlite.DatasetTransaction) error) error
}

func (source ruleSyncConfigSource) authors(ctx context.Context) (string, error) {
	value, err := source.store.Repositories().SystemConfigs.ValueByKey(ctx, "ruleSyncAuthors")
	if err != nil {
		return "", fmt.Errorf("load rule sync authors: %w", err)
	}
	return value, nil
}

type liveRuleSyncer struct {
	config     ruleSyncConfigSource
	store      ruleTransactionStore
	domain     rulesync.Domain
	client     *http.Client
	timeout    time.Duration
	primary    string
	backup     string
	invalidate func(context.Context, string) error
}

func (syncer liveRuleSyncer) Sync(ctx context.Context) (rule.SyncResult, error) {
	authors, err := syncer.config.authors(ctx)
	if err != nil {
		return rule.SyncSucceeded, err
	}
	client, err := rulesync.NewClient(rulesync.ClientOptions{PrimaryURL: syncer.primary, BackupURL: syncer.backup, Timeout: syncer.timeout, HTTPClient: syncer.client})
	if err != nil {
		return rule.SyncSucceeded, fmt.Errorf("create rule sync client: %w", err)
	}
	service := rulesync.NewService(rulesync.ServiceDependencies{Client: client, Store: syncer.store, Authors: authors})
	var report rulesync.Report
	if syncer.domain == rulesync.Sonarr {
		report, err = service.SyncSonarr(ctx)
	} else {
		report, err = service.SyncRadarr(ctx)
	}
	if err != nil {
		return rule.SyncSucceeded, fmt.Errorf("sync %s rules: %w", syncer.domain, err)
	}
	if !report.Succeeded() {
		if hasCommittedRules(report) && syncer.invalidate != nil {
			if err := syncer.invalidate(ctx, ruleScope(syncer.domain)); err != nil {
				return rule.SyncSucceeded, fmt.Errorf("refresh committed %s rules: %w", syncer.domain, err)
			}
		}
		return rule.SyncSucceeded, errPartialRuleSync
	}
	return rule.SyncSucceeded, nil
}

func hasCommittedRules(report rulesync.Report) bool {
	for _, author := range report.Authors() {
		if author.Committed > 0 {
			return true
		}
	}
	return false
}

func ruleScope(domain rulesync.Domain) string {
	if domain == rulesync.Radarr {
		return "radarr_rule"
	}
	return "sonarr_rule"
}

func liveRuleSyncDependencies(cfg config.Config, store managementStore, invalidate func(context.Context, string) error) ruleSyncDependencies {
	transactionStore, ok := store.(ruleTransactionStore)
	if !ok {
		return ruleSyncDependencies{}
	}
	primary, backup := system.DefaultRuleSyncSources()
	client := &http.Client{Timeout: cfg.HTTPTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return ruleSyncDependencies{
		sonarr: liveRuleSyncer{config: ruleSyncConfigSource{store}, store: transactionStore, domain: rulesync.Sonarr, client: client, timeout: cfg.HTTPTimeout, primary: primary, backup: backup, invalidate: invalidate},
		radarr: liveRuleSyncer{config: ruleSyncConfigSource{store}, store: transactionStore, domain: rulesync.Radarr, client: client, timeout: cfg.HTTPTimeout, primary: primary, backup: backup, invalidate: invalidate},
	}
}
