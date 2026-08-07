package rename

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestService_RunSonarr_stopsBeforeNextEventWhenCancelled(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	hashOne := "abcdef0123456789abcdef0123456789abcdef01"
	hashTwo := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	history := fakeHistory{events: []Event{{SourceTitle: "first", Hash: hashOne, Downloader: DownloaderQBittorrent}, {SourceTitle: "second", Hash: hashTwo, Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{cancel: cancel}
	service, err := NewService(ServiceOptions{History: history, QBittorrent: qb, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunSonarr(ctx, Endpoint{}, time.Minute)

	// Then
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(qb.renames, []string{hashOne + ":first"}) {
		t.Fatalf("err=%v renames=%#v", err, qb.renames)
	}
}

func TestService_RunConfiguredSonarr_loadsEndpointForEachRun(t *testing.T) {
	// Given
	config := &fakeConfig{values: map[string]string{sonarrURLKey: "https://sonarr.test", sonarrAPIKeyKey: "key"}}
	history := &recordingHistory{}
	service, err := NewService(ServiceOptions{History: history, Config: config, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	if err := service.RunConfiguredSonarr(context.Background(), time.Minute); err != nil {
		t.Fatal(err)
	}
	config.values[sonarrAPIKeyKey] = "next"
	if err := service.RunConfiguredSonarr(context.Background(), time.Minute); err != nil {
		t.Fatal(err)
	}

	// Then
	if !reflect.DeepEqual(history.keys, []string{"key", "next"}) || config.calls != 4 {
		t.Fatalf("history keys=%#v config calls=%d", history.keys, config.calls)
	}
}

func TestLoginService_Startup_attemptsBothAndRefreshOnlyQBittorrent(t *testing.T) {
	// Given
	qb := &fakeLogin{err: errors.New("unavailable")}
	transmission := &fakeLogin{}
	service := LoginService{QBittorrent: qb, Transmission: transmission}

	// When
	service.Startup(context.Background())
	service.RefreshQBittorrent(context.Background())

	// Then
	if qb.calls != 2 || transmission.calls != 1 {
		t.Fatalf("login calls qBittorrent=%d transmission=%d", qb.calls, transmission.calls)
	}
}
