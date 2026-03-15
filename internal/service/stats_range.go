package service

import (
	"strings"
	"time"

	"slink-api/internal/dto"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/response"
)

type resolvedStatsRange struct {
	Period      string
	Start       time.Time
	End         time.Time
	Granularity string
}

func resolveStatsRange(req dto.StatsRangeRequest, now time.Time, defaultPeriod string) (*resolvedStatsRange, error) {
	period := strings.ToLower(strings.TrimSpace(req.RequestedPeriod()))
	startInput := strings.TrimSpace(req.RequestedStart())
	endInput := strings.TrimSpace(req.RequestedEnd())

	if period == "" && (startInput != "" || endInput != "") {
		period = "custom"
	}
	if period == "" {
		period = defaultPeriod
	}
	if period == "" {
		period = "7d"
	}

	end := now
	start := dayStart(now.AddDate(0, 0, -6))
	var err error

	switch period {
	case "today":
		start = dayStart(now)
		end = now
	case "24h":
		start = now.Add(-24 * time.Hour)
		end = now
	case "7d":
		start = dayStart(now.AddDate(0, 0, -6))
		end = now
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = now
	case "30d":
		start = dayStart(now.AddDate(0, 0, -29))
		end = now
	case "90d":
		start = dayStart(now.AddDate(0, 0, -89))
		end = now
	case "all":
		start = time.Time{}
		end = now
	case "custom":
		if startInput == "" && endInput == "" {
			return nil, bizErrors.New(response.InvalidParam, "custom time range requires start_date/end_date")
		}
		if endInput != "" {
			end, err = parseFlexibleDate(endInput)
			if err != nil {
				return nil, bizErrors.New(response.InvalidParam, "invalid end_date")
			}
		}
		start = dayStart(end.AddDate(0, 0, -6))
		if startInput != "" {
			start, err = parseFlexibleDate(startInput)
			if err != nil {
				return nil, bizErrors.New(response.InvalidParam, "invalid start_date")
			}
		}
	default:
		return nil, bizErrors.New(response.InvalidParam, "invalid period")
	}

	if start.After(end) {
		start, end = end, start
	}

	granularity := strings.ToLower(strings.TrimSpace(req.Granularity))
	if granularity == "" || granularity == "auto" {
		granularity = autoGranularity(period, start, end)
	}

	return &resolvedStatsRange{
		Period:      period,
		Start:       start,
		End:         end,
		Granularity: granularity,
	}, nil
}

func autoGranularity(period string, start, end time.Time) string {
	switch period {
	case "today", "24h":
		return "hour"
	}
	if !start.IsZero() && end.Sub(start) <= 24*time.Hour {
		return "hour"
	}
	return "day"
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func dayEnd(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

func normalizeDateRange(start, end time.Time) (time.Time, time.Time) {
	if start.IsZero() {
		return start, dayEnd(end)
	}
	return dayStart(start), dayEnd(end)
}

func clampRangeStart(start, min time.Time) time.Time {
	if start.IsZero() {
		return min
	}
	if start.Before(min) {
		return min
	}
	return start
}

func parseFlexibleDate(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	var err error
	for _, layout := range layouts {
		var parsed time.Time
		parsed, err = time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, err
}
