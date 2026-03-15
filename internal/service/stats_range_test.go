package service

import (
	"testing"
	"time"

	"slink-api/internal/dto"
)

func TestResolveStatsRangePresets(t *testing.T) {
	now := time.Date(2026, 3, 14, 15, 30, 0, 0, time.Local)

	tests := []struct {
		name          string
		req           dto.StatsRangeRequest
		defaultPeriod string
		wantPeriod    string
		wantStart     time.Time
		wantEnd       time.Time
		wantGran      string
	}{
		{
			name:          "default 7d",
			req:           dto.StatsRangeRequest{},
			defaultPeriod: "7d",
			wantPeriod:    "7d",
			wantStart:     time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local),
			wantEnd:       now,
			wantGran:      "day",
		},
		{
			name:          "today uses hour granularity",
			req:           dto.StatsRangeRequest{Period: "today"},
			defaultPeriod: "7d",
			wantPeriod:    "today",
			wantStart:     time.Date(2026, 3, 14, 0, 0, 0, 0, time.Local),
			wantEnd:       now,
			wantGran:      "hour",
		},
		{
			name:          "range alias",
			req:           dto.StatsRangeRequest{Range: "30d"},
			defaultPeriod: "7d",
			wantPeriod:    "30d",
			wantStart:     time.Date(2026, 2, 13, 0, 0, 0, 0, time.Local),
			wantEnd:       now,
			wantGran:      "day",
		},
		{
			name:          "custom with explicit dates",
			req:           dto.StatsRangeRequest{StartDate: "2026-03-01", EndDate: "2026-03-03"},
			defaultPeriod: "7d",
			wantPeriod:    "custom",
			wantStart:     time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local),
			wantEnd:       time.Date(2026, 3, 3, 0, 0, 0, 0, time.Local),
			wantGran:      "day",
		},
		{
			name:          "custom short span uses hour",
			req:           dto.StatsRangeRequest{StartDate: "2026-03-13 12:00", EndDate: "2026-03-14 11:00"},
			defaultPeriod: "7d",
			wantPeriod:    "custom",
			wantStart:     time.Date(2026, 3, 13, 12, 0, 0, 0, time.Local),
			wantEnd:       time.Date(2026, 3, 14, 11, 0, 0, 0, time.Local),
			wantGran:      "hour",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveStatsRange(tt.req, now, tt.defaultPeriod)
			if err != nil {
				t.Fatalf("resolveStatsRange returned error: %v", err)
			}
			if got.Period != tt.wantPeriod {
				t.Fatalf("period mismatch: got %q want %q", got.Period, tt.wantPeriod)
			}
			if !got.Start.Equal(tt.wantStart) {
				t.Fatalf("start mismatch: got %s want %s", got.Start, tt.wantStart)
			}
			if !got.End.Equal(tt.wantEnd) {
				t.Fatalf("end mismatch: got %s want %s", got.End, tt.wantEnd)
			}
			if got.Granularity != tt.wantGran {
				t.Fatalf("granularity mismatch: got %q want %q", got.Granularity, tt.wantGran)
			}
		})
	}
}

func TestResolveStatsRangeCustomRequiresBounds(t *testing.T) {
	_, err := resolveStatsRange(dto.StatsRangeRequest{Period: "custom"}, time.Now(), "7d")
	if err == nil {
		t.Fatal("expected error for custom period without bounds")
	}
}

func TestNormalizeDateRange(t *testing.T) {
	start := time.Date(2026, 3, 14, 15, 30, 0, 0, time.Local)
	end := time.Date(2026, 3, 15, 10, 45, 0, 0, time.Local)

	gotStart, gotEnd := normalizeDateRange(start, end)

	wantStart := time.Date(2026, 3, 14, 0, 0, 0, 0, time.Local)
	wantEnd := time.Date(2026, 3, 15, 23, 59, 59, int(time.Second-time.Nanosecond), time.Local)

	if !gotStart.Equal(wantStart) {
		t.Fatalf("start mismatch: got %s want %s", gotStart, wantStart)
	}
	if !gotEnd.Equal(wantEnd) {
		t.Fatalf("end mismatch: got %s want %s", gotEnd, wantEnd)
	}
}
