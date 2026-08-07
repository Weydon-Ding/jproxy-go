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
	hashOne := "abcdef0123456789abcdef0123456789abcdef01"
	hashTwo := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	history := fakeHistory{events: []Event{{SourceTitle: "first", Hash: hashOne, Downloader: DownloaderQBittorrent}, {SourceTitle: "duplicate", Hash: hashOne, Downloader: DownloaderQBittorrent}, {SourceTitle: "second", Hash: hashTwo, Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{renameErrors: map[string]error{hashOne: errors.New("missing")}}
	service, err := NewService(ServiceOptions{History: history, QBittorrent: qb, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunSonarr(context.Background(), Endpoint{URL: "https://sonarr.test", APIKey: "key"}, time.Minute)

	// Then
	if !errors.Is(err, qb.renameErrors[hashOne]) || !reflect.DeepEqual(qb.renames, []string{hashOne + ":first", hashTwo + ":second"}) {
		t.Fatalf("err=%v renames=%#v", err, qb.renames)
	}
}

func TestService_RunRadarr_joinsTransmissionFailuresAndContinues(t *testing.T) {
	// Given
	failure := errors.New("unavailable")
	hashOne := "abcdef0123456789abcdef0123456789abcdef01"
	hashTwo := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	history := fakeHistory{events: []Event{{SourceTitle: "first", Hash: hashOne, Downloader: DownloaderTransmission}, {SourceTitle: "second", Hash: hashTwo, Downloader: DownloaderTransmission}}}
	transmission := &fakeTorrent{errors: map[string]error{hashOne: failure}}
	service, err := NewService(ServiceOptions{History: history, Transmission: transmission, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunRadarr(context.Background(), Endpoint{}, time.Minute)

	// Then
	if !errors.Is(err, failure) || !reflect.DeepEqual(transmission.renames, []string{hashOne + ":first", hashTwo + ":second"}) {
		t.Fatalf("err=%v renames=%#v", err, transmission.renames)
	}
}

func TestService_RunRadarr_preservesEventMultiplicityAndTransmissionOnlyRenamesTorrent(t *testing.T) {
	// Given
	hash := "abcdef0123456789abcdef0123456789abcdef01"
	history := fakeHistory{events: []Event{{SourceTitle: "movie", Hash: hash, Downloader: DownloaderTransmission}, {SourceTitle: "again", Hash: hash, Downloader: DownloaderTransmission}}}
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
	if !reflect.DeepEqual(transmission.renames, []string{hash + ":movie", hash + ":again"}) {
		t.Fatalf("renames = %#v", transmission.renames)
	}
}

func TestService_RunRadarr_skipsInvalidEventsWithoutMutation(t *testing.T) {
	// Given
	validHash := "abcdef0123456789abcdef0123456789abcdef01"
	history := fakeHistory{events: []Event{
		{SourceTitle: "SAB", Hash: validHash, Downloader: Downloader("sabnzbd")},
		{SourceTitle: "NZB", Hash: validHash, Downloader: Downloader("nzbget")},
		{SourceTitle: "Blank", Hash: validHash, Downloader: Downloader("")},
		{SourceTitle: "Typo", Hash: validHash, Downloader: Downloader("transmis" + "ion")},
		{SourceTitle: "Short", Hash: validHash[:39], Downloader: DownloaderTransmission},
		{SourceTitle: "Long", Hash: validHash + "a", Downloader: DownloaderTransmission},
		{SourceTitle: "Nonhex", Hash: validHash[:39] + "z", Downloader: DownloaderTransmission},
	}}
	transmission := &fakeTorrent{}
	service, err := NewService(ServiceOptions{History: history, Transmission: transmission, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunRadarr(context.Background(), Endpoint{}, time.Minute)

	// Then
	if err != nil || len(transmission.renames) != 0 {
		t.Fatalf("err=%v renames=%#v", err, transmission.renames)
	}
}

func TestService_RunRadarr_passesTransmissionTitlesWithoutFileRenames(t *testing.T) {
	// Given
	hash := "abcdef0123456789abcdef0123456789abcdef01"
	cases := []string{"Movie: Director", "Movie"}
	for _, test := range cases {
		t.Run(test, func(t *testing.T) {
			// Given
			transmission := &fakeTorrent{}
			service, err := NewService(ServiceOptions{History: fakeHistory{events: []Event{{SourceTitle: test, Hash: hash, Downloader: DownloaderTransmission}}}, Transmission: transmission, Now: fixedNow})
			if err != nil {
				t.Fatal(err)
			}

			// When
			err = service.RunRadarr(context.Background(), Endpoint{}, time.Minute)

			// Then
			if err != nil || !reflect.DeepEqual(transmission.renames, []string{hash + ":" + test}) {
				t.Fatalf("err=%v renames=%#v", err, transmission.renames)
			}
		})
	}
}

func TestService_RunRadarr_usesMovieFilePlanWhenSonarrFormatIsConfigured(t *testing.T) {
	// Given
	hash := "abcdef0123456789abcdef0123456789abcdef01"
	history := fakeHistory{events: []Event{{SourceTitle: "Movie", Hash: hash, Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{files: map[string][]string{hash: {"old/file.mkv"}}}
	service, err := NewService(ServiceOptions{History: history, QBittorrent: qb, Files: FileOptions{Enabled: true, SonarrFormat: "{season}{episode}"}, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	if err := service.RunRadarr(context.Background(), Endpoint{URL: "https://radarr.test", APIKey: "key"}, time.Minute); err != nil {
		t.Fatal(err)
	}

	// Then
	if !reflect.DeepEqual(qb.fileRenames, []string{hash + ":old/file.mkv:Movie/Movie.mkv"}) {
		t.Fatalf("file mutations=%#v", qb.fileRenames)
	}
}

func TestService_RunSonarr_makesNoMutationWhenFilePlanIsInvalid(t *testing.T) {
	// Given
	hash := "abcdef0123456789abcdef0123456789abcdef01"
	history := fakeHistory{events: []Event{{SourceTitle: "release", Hash: hash, Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{files: map[string][]string{hash: {"../escape.mkv"}}}
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
	hashOne := "abcdef0123456789abcdef0123456789abcdef01"
	hashTwo := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	history := fakeHistory{events: []Event{{SourceTitle: "first", Hash: hashOne, Downloader: DownloaderQBittorrent}, {SourceTitle: "second", Hash: hashTwo, Downloader: DownloaderQBittorrent}}}
	qb := &fakeQB{files: map[string][]string{hashOne: {"old/one.mkv", "old/two.mkv"}, hashTwo: {"other/file.mkv"}}, fileRenameErrors: map[string]error{"old/one.mkv": failure}}
	service, err := NewService(ServiceOptions{History: history, QBittorrent: qb, Files: FileOptions{Enabled: true}, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = service.RunSonarr(context.Background(), Endpoint{URL: "https://sonarr.test", APIKey: "key"}, time.Minute)

	// Then
	wantFiles := []string{hashOne + ":old/one.mkv:first/first.mkv", hashTwo + ":other/file.mkv:second/second.mkv"}
	if !errors.Is(err, failure) || !reflect.DeepEqual(qb.renames, []string{hashOne + ":first", hashTwo + ":second"}) || !reflect.DeepEqual(qb.fileRenames, wantFiles) {
		t.Fatalf("err=%v torrent mutations=%#v file mutations=%#v", err, qb.renames, qb.fileRenames)
	}
}
