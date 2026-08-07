package rename

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestService_RunSonarr_deduplicatesHashesAndContinuesAfterFailures(t *testing.T) {
	// Given
	history := fakeHistory{events: []Event{{SourceTitle: "first", Hash: "abc", Downloader: DownloaderQBittorrent}, {SourceTitle: "duplicate", Hash: "abc", Downloader: DownloaderQBittorrent}, {SourceTitle: "second", Hash: "def", Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{renameErrors: map[string]error{"abc": errors.New("missing")}}
	service, err := NewService(ServiceOptions{History: history, QBittorrent: qb, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunSonarr(context.Background(), Endpoint{URL: "https://sonarr.test", APIKey: "key"}, time.Minute)

	// Then
	if !errors.Is(err, qb.renameErrors["abc"]) || !reflect.DeepEqual(qb.renames, []string{"abc:first", "def:second"}) {
		t.Fatalf("err=%v renames=%#v", err, qb.renames)
	}
}

func TestService_RunRadarr_joinsTransmissionFailuresAndContinues(t *testing.T) {
	// Given
	failure := errors.New("unavailable")
	history := fakeHistory{events: []Event{{SourceTitle: "first", Hash: "one", Downloader: DownloaderTransmission}, {SourceTitle: "second", Hash: "two", Downloader: DownloaderTransmission}}}
	transmission := &fakeTorrent{errors: map[string]error{"one": failure}}
	service, err := NewService(ServiceOptions{History: history, Transmission: transmission, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunRadarr(context.Background(), Endpoint{}, time.Minute)

	// Then
	if !errors.Is(err, failure) || !reflect.DeepEqual(transmission.renames, []string{"one:first", "two:second"}) {
		t.Fatalf("err=%v renames=%#v", err, transmission.renames)
	}
}

func TestService_RunRadarr_preservesEventMultiplicityAndTransmissionOnlyRenamesTorrent(t *testing.T) {
	// Given
	history := fakeHistory{events: []Event{{SourceTitle: "movie", Hash: "ABC", Downloader: DownloaderTransmission}, {SourceTitle: "again", Hash: "abc", Downloader: DownloaderTransmission}}}
	transmission := &fakeTorrent{}
	service, err := NewService(ServiceOptions{History: history, Transmission: transmission, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	if err := service.RunRadarr(context.Background(), Endpoint{URL: "https://radarr.test", APIKey: "key"}, time.Minute); err != nil {
		t.Fatal(err)
	}

	// Then
	if !reflect.DeepEqual(transmission.renames, []string{"ABC:movie", "abc:again"}) {
		t.Fatalf("renames = %#v", transmission.renames)
	}
}

func TestService_RunRadarr_usesMovieFilePlanWhenSonarrFormatIsConfigured(t *testing.T) {
	// Given
	history := fakeHistory{events: []Event{{SourceTitle: "Movie", Hash: "abc", Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{files: map[string][]string{"abc": {"old/file.mkv"}}}
	service, err := NewService(ServiceOptions{History: history, QBittorrent: qb, Files: FileOptions{Enabled: true, SonarrFormat: "{season}{episode}"}, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	if err := service.RunRadarr(context.Background(), Endpoint{URL: "https://radarr.test", APIKey: "key"}, time.Minute); err != nil {
		t.Fatal(err)
	}

	// Then
	if !reflect.DeepEqual(qb.fileRenames, []string{"abc:old/file.mkv:Movie/Movie.mkv"}) {
		t.Fatalf("file mutations=%#v", qb.fileRenames)
	}
}

func TestService_RunSonarr_makesNoMutationWhenFilePlanIsInvalid(t *testing.T) {
	// Given
	history := fakeHistory{events: []Event{{SourceTitle: "release", Hash: "abc", Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{files: map[string][]string{"abc": {"../escape.mkv"}}}
	service, err := NewService(ServiceOptions{History: history, QBittorrent: qb, Files: FileOptions{Enabled: true}, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunSonarr(context.Background(), Endpoint{URL: "https://sonarr.test", APIKey: "key"}, time.Minute)

	// Then
	if !errors.Is(err, ErrInvalidPath) || len(qb.renames) != 0 || len(qb.fileRenames) != 0 {
		t.Fatalf("err=%v torrent mutations=%#v file mutations=%#v", err, qb.renames, qb.fileRenames)
	}
}

func TestService_RunSonarr_stopsFilesForFailedEventThenContinuesIndependentEvent(t *testing.T) {
	// Given
	failure := errors.New("missing")
	history := fakeHistory{events: []Event{{SourceTitle: "first", Hash: "abc", Downloader: DownloaderQBittorrent}, {SourceTitle: "second", Hash: "def", Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{files: map[string][]string{"abc": {"old/one.mkv", "old/two.mkv"}, "def": {"other/file.mkv"}}, fileRenameErrors: map[string]error{"old/one.mkv": failure}}
	service, err := NewService(ServiceOptions{History: history, QBittorrent: qb, Files: FileOptions{Enabled: true}, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunSonarr(context.Background(), Endpoint{URL: "https://sonarr.test", APIKey: "key"}, time.Minute)

	// Then
	wantFiles := []string{"abc:old/one.mkv:first/first.mkv", "def:other/file.mkv:second/second.mkv"}
	if !errors.Is(err, failure) || !reflect.DeepEqual(qb.renames, []string{"abc:first", "def:second"}) || !reflect.DeepEqual(qb.fileRenames, wantFiles) {
		t.Fatalf("err=%v torrent mutations=%#v file mutations=%#v", err, qb.renames, qb.fileRenames)
	}
}
