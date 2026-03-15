package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockStatsRepository(t *testing.T) (*statsRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("failed to open gorm db: %v", err)
	}

	cleanup := func() {
		_ = sqlDB.Close()
	}

	return &statsRepository{db: db}, mock, cleanup
}

func TestGetClicksByDateReturnsZeroWhenAggregateIsEmpty(t *testing.T) {
	repo, mock, cleanup := newMockStatsRepository(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(clicks), 0) FROM `stats_daily` WHERE short_code = ? AND date = ?")).
		WithArgs("52KD2a", "2026-03-14").
		WillReturnRows(sqlmock.NewRows([]string{"clicks"}).AddRow(0))

	clicks, err := repo.GetClicksByDate(t.Context(), "52KD2a", "2026-03-14")
	if err != nil {
		t.Fatalf("GetClicksByDate returned error: %v", err)
	}
	if clicks != 0 {
		t.Fatalf("expected clicks to be 0, got %d", clicks)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetUserTrendByHourFiltersByShortlinkOwner(t *testing.T) {
	repo, mock, cleanup := newMockStatsRepository(t)
	defer cleanup()

	start := time.Date(2026, 3, 14, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 3, 14, 23, 59, 59, 0, time.Local)

	mock.ExpectQuery(regexp.QuoteMeta("JOIN shortlinks s ON al.short_code = s.short_code")).
		WithArgs(uint(7), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"time", "count"}).
			AddRow("2026-03-14 09:00:00", 12).
			AddRow("2026-03-14 10:00:00", 8))

	points, err := repo.GetUserTrendByHour(t.Context(), 7, start, end)
	if err != nil {
		t.Fatalf("GetUserTrendByHour returned error: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 trend points, got %d", len(points))
	}
	if points[0].Time != "2026-03-14 09:00:00" || points[0].Count != 12 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
	if points[1].Time != "2026-03-14 10:00:00" || points[1].Count != 8 {
		t.Fatalf("unexpected second point: %+v", points[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetUserSourcesFiltersByShortlinkOwner(t *testing.T) {
	repo, mock, cleanup := newMockStatsRepository(t)
	defer cleanup()

	start := time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 3, 14, 23, 59, 59, 0, time.Local)

	mock.ExpectQuery(regexp.QuoteMeta("JOIN shortlinks s ON al.short_code = s.short_code")).
		WithArgs(uint(7), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"name", "value"}).
			AddRow("weixin.qq.com", 19).
			AddRow("zhihu.com", 6))

	sources, err := repo.GetUserSources(t.Context(), 7, start, end)
	if err != nil {
		t.Fatalf("GetUserSources returned error: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Name != "weixin.qq.com" || sources[0].Value != 19 {
		t.Fatalf("unexpected first source: %+v", sources[0])
	}
	if sources[1].Name != "zhihu.com" || sources[1].Value != 6 {
		t.Fatalf("unexpected second source: %+v", sources[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetUserCitiesAllowsGlobalFallbackWhenProvinceIsEmpty(t *testing.T) {
	repo, mock, cleanup := newMockStatsRepository(t)
	defer cleanup()

	start := time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 3, 14, 0, 0, 0, 0, time.Local)

	mock.ExpectQuery(regexp.QuoteMeta("JOIN shortlinks s ON srd.short_code = s.short_code")).
		WithArgs(uint(7), start, end, 10).
		WillReturnRows(sqlmock.NewRows([]string{"name", "value"}).
			AddRow("Beijing", 14).
			AddRow("Shenzhen", 11))

	cities, err := repo.GetUserCities(t.Context(), 7, "", start, end)
	if err != nil {
		t.Fatalf("GetUserCities returned error: %v", err)
	}
	if len(cities) != 2 {
		t.Fatalf("expected 2 cities, got %d", len(cities))
	}
	if cities[0].Name != "Beijing" || cities[0].Value != 14 {
		t.Fatalf("unexpected first city: %+v", cities[0])
	}
	if cities[1].Name != "Shenzhen" || cities[1].Value != 11 {
		t.Fatalf("unexpected second city: %+v", cities[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestNormalizeSourceNameStripsWWWPrefix(t *testing.T) {
	if got := normalizeSourceName("www.zhihu.com"); got != "zhihu.com" {
		t.Fatalf("expected zhihu.com, got %s", got)
	}

	if got := normalizeSourceName("https://www.google.com/search?q=slink"); got != "google.com" {
		t.Fatalf("expected google.com, got %s", got)
	}

	if got := normalizeSourceName("weixin.qq.com"); got != "weixin.qq.com" {
		t.Fatalf("expected weixin.qq.com, got %s", got)
	}
}
