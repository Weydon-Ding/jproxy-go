package tasks

import (
	"errors"
	"testing"
	"time"
)

func TestSchedule_Next_matchesSevenProductionSchedulesAtExactBoundary(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	tests := []struct {
		name        string
		newSchedule func() (Schedule, error)
		from        time.Time
		want        time.Time
	}{
		{"sonarr title hourly", func() (Schedule, error) { return Hourly(0, 0) }, at(location, 2026, 8, 7, 9, 0, 0), at(location, 2026, 8, 7, 10, 0, 0)},
		{"sonarr rule daily", func() (Schedule, error) { return Daily(0, 15, 0) }, at(location, 2026, 8, 7, 0, 15, 0), at(location, 2026, 8, 8, 0, 15, 0)},
		{"sonarr rename every thirty seconds", func() (Schedule, error) { return EverySeconds(30) }, at(location, 2026, 8, 7, 9, 0, 30), at(location, 2026, 8, 7, 9, 1, 0)},
		{"radarr title hourly at minute thirty", func() (Schedule, error) { return Hourly(30, 0) }, at(location, 2026, 8, 7, 9, 30, 0), at(location, 2026, 8, 7, 10, 30, 0)},
		{"radarr rule daily", func() (Schedule, error) { return Daily(1, 45, 0) }, at(location, 2026, 8, 7, 1, 45, 0), at(location, 2026, 8, 8, 1, 45, 0)},
		{"radarr rename every minute", func() (Schedule, error) { return EveryMinutes(1, 0) }, at(location, 2026, 8, 7, 9, 1, 0), at(location, 2026, 8, 7, 9, 2, 0)},
		{"downloader login every thirty minutes", func() (Schedule, error) { return EveryMinutes(30, 0) }, at(location, 2026, 8, 7, 9, 30, 0), at(location, 2026, 8, 7, 10, 0, 0)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			schedule, err := test.newSchedule()
			if err != nil {
				t.Fatalf("construct schedule: %v", err)
			}

			// When
			got := schedule.Next(test.from)

			// Then
			if !got.Equal(test.want) {
				t.Fatalf("Next() = %s, want %s", got, test.want)
			}
			if !got.After(test.from) {
				t.Fatalf("Next() = %s must be after %s", got, test.from)
			}
		})
	}
}

func TestSchedule_Next_normalizesDayAndMonthWithTimeDate(t *testing.T) {
	// Given
	schedule, err := Daily(0, 15, 0)
	if err != nil {
		t.Fatalf("Daily(): %v", err)
	}
	location := time.FixedZone("UTC+8", 8*60*60)

	// When
	got := schedule.Next(at(location, 2026, 1, 31, 23, 59, 59))

	// Then
	want := at(location, 2026, 2, 1, 0, 15, 0)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want %s", got, want)
	}
}

func TestDaily_Next_preservesLocalBoundaryAcrossDSTFallback(t *testing.T) {
	// Given
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	schedule, err := Daily(0, 15, 0)
	if err != nil {
		t.Fatalf("Daily(): %v", err)
	}
	from := at(location, 2026, 11, 1, 0, 15, 0)

	// When
	got := schedule.Next(from)

	// Then
	want := at(location, 2026, 11, 2, 0, 15, 0)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want %s", got, want)
	}
	if got.Sub(from) != 25*time.Hour {
		t.Fatalf("boundary duration = %s, want 25h across DST fallback", got.Sub(from))
	}
}

func TestHourly_Next_returnsSecondRepeatedHourOccurrence(t *testing.T) {
	// Given
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	schedule, err := Hourly(30, 0)
	if err != nil {
		t.Fatalf("Hourly(): %v", err)
	}
	from := time.Date(2026, 11, 1, 5, 45, 0, 0, time.UTC).In(location)

	// When
	got := schedule.Next(from)

	// Then
	want := time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC).In(location)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want repeated occurrence %s", got, want)
	}
}

func TestEveryMinutes_Next_returnsSecondRepeatedHourOccurrence(t *testing.T) {
	// Given
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	schedule, err := EveryMinutes(30, 0)
	if err != nil {
		t.Fatalf("EveryMinutes(): %v", err)
	}
	from := time.Date(2026, 11, 1, 5, 45, 0, 0, time.UTC).In(location)

	// When
	got := schedule.Next(from)

	// Then
	want := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC).In(location)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want repeated occurrence %s", got, want)
	}
}

func TestHourly_Next_skipsNonexistentSpringForwardBoundary(t *testing.T) {
	// Given
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	schedule, err := Hourly(30, 0)
	if err != nil {
		t.Fatalf("Hourly(): %v", err)
	}
	from := time.Date(2026, 3, 8, 1, 45, 0, 0, location)

	// When
	got := schedule.Next(from)

	// Then
	want := time.Date(2026, 3, 8, 3, 30, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want next valid boundary %s", got, want)
	}
}

func TestEveryMinutes_Next_skipsNonexistentSpringForwardBoundary(t *testing.T) {
	// Given
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	schedule, err := EveryMinutes(30, 0)
	if err != nil {
		t.Fatalf("EveryMinutes(): %v", err)
	}
	from := time.Date(2026, 3, 8, 1, 45, 0, 0, location)

	// When
	got := schedule.Next(from)

	// Then
	want := time.Date(2026, 3, 8, 3, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want next valid boundary %s", got, want)
	}
}

func TestEveryMinutes_Next_honorsSecondWithinCurrentInterval(t *testing.T) {
	// Given
	schedule, err := EveryMinutes(30, 15)
	if err != nil {
		t.Fatalf("EveryMinutes(): %v", err)
	}
	location := time.FixedZone("UTC+8", 8*60*60)

	// When
	got := schedule.Next(at(location, 2026, 8, 7, 9, 0, 0))

	// Then
	want := at(location, 2026, 8, 7, 9, 0, 15)
	if !got.Equal(want) {
		t.Fatalf("Next() = %s, want %s", got, want)
	}
}

func TestSchedule_ConstructorsRejectInvalidFields(t *testing.T) {
	tests := []struct {
		name      string
		construct func() (Schedule, error)
	}{
		{"daily hour", func() (Schedule, error) { return Daily(24, 0, 0) }},
		{"daily minute", func() (Schedule, error) { return Daily(0, 60, 0) }},
		{"daily second", func() (Schedule, error) { return Daily(0, 0, 60) }},
		{"hourly minute", func() (Schedule, error) { return Hourly(60, 0) }},
		{"every minutes interval", func() (Schedule, error) { return EveryMinutes(0, 0) }},
		{"every minutes divisor", func() (Schedule, error) { return EveryMinutes(7, 0) }},
		{"every seconds interval", func() (Schedule, error) { return EverySeconds(0) }},
		{"every seconds divisor", func() (Schedule, error) { return EverySeconds(7) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, err := test.construct()

			// Then
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func at(location *time.Location, year int, month time.Month, day, hour, minute, second int) time.Time {
	return time.Date(year, month, day, hour, minute, second, 0, location)
}
