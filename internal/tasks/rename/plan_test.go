package rename

import (
	"reflect"
	"testing"

	"jproxy-go/internal/format"
)

func TestPlanSonarrFiles_returnsNoPlanWhenAnyPathIsInvalid(t *testing.T) {
	// Given
	rules := []format.Rule{{Token: "season", Regex: `S(\d+)`, Replacement: "S$1"}, {Token: "episode", Regex: `E(\d+)`, Replacement: "E$1"}}
	files := []string{"old/Show.S01E02.mkv", "../escape.srt"}

	// When
	plan, err := PlanSonarrFiles("Release [WEB]", files, "{season}{episode}", rules)

	// Then
	if err == nil || plan != nil {
		t.Fatalf("PlanSonarrFiles() = %#v, %v", plan, err)
	}
}

func TestPlanSonarrFiles_preservesSubdirectoriesSuffixAndSubtitleNumbers(t *testing.T) {
	// Given
	rules := []format.Rule{{Token: "season", Regex: `.*S(\d+).*`, Replacement: "S$1"}, {Token: "episode", Regex: `.*E(\d+).*`, Replacement: "E$1"}}
	files := []string{"old/Show.S01E02.mkv", "old/sub/Show.S01E02.eng.srt", "old/sub/Show.S01E02.chs.srt"}

	// When
	plan, err := PlanSonarrFiles("Release [WEB]", files, "{season}{episode}", rules)

	// Then
	want := []FileRename{
		{OldPath: "old/Show.S01E02.mkv", NewPath: "Release [WEB]/S01E02 [WEB].mkv"},
		{OldPath: "old/sub/Show.S01E02.eng.srt", NewPath: "Release [WEB]/sub/S01E02 [WEB].1.eng.srt"},
		{OldPath: "old/sub/Show.S01E02.chs.srt", NewPath: "Release [WEB]/sub/S01E02 [WEB].2.chs.srt"},
	}
	if err != nil || !reflect.DeepEqual(plan, want) {
		t.Fatalf("PlanSonarrFiles() = %#v, %v", plan, err)
	}
}

func TestPlanRadarrFiles_stopsOnlyWhenTheCompleteParentDirectoryAlreadyMatches(t *testing.T) {
	// Given
	files := []string{"Release/sub/file.mkv", "Release [WEB]/file.mkv"}

	// When
	plan, err := PlanRadarrFiles("Release", files)

	// Then
	want := []FileRename{{OldPath: "Release/sub/file.mkv", NewPath: "Release/sub/Release.mkv"}, {OldPath: "Release [WEB]/file.mkv", NewPath: "Release/Release.mkv"}}
	if err != nil || !reflect.DeepEqual(plan, want) {
		t.Fatalf("PlanRadarrFiles() = %#v, %v", plan, err)
	}
}
