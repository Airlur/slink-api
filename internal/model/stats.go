package model

import "time"

// StatsDaily ��Ӧ���ݿ��е� stats_daily ���
type StatsDaily struct {
	ID        uint      `gorm:"primarykey"`
	ShortCode string    `gorm:"column:short_code;type:varchar(20);not null;uniqueIndex:uk_code_date"`
	Date      time.Time `gorm:"column:date;type:date;not null;uniqueIndex:uk_code_date"`
	Clicks    uint      `gorm:"column:clicks;not null"`
}

func (StatsDaily) TableName() string {
	return "stats_daily"
}

// StatsRegionDaily ��Ӧ���ݿ��е� stats_region_daily ���
type StatsRegionDaily struct {
	ID        uint      `gorm:"primarykey"`
	ShortCode string    `gorm:"column:short_code;type:varchar(20);not null;uniqueIndex:uk_code_date_country_province_city"`
	Date      time.Time `gorm:"column:date;type:date;not null;uniqueIndex:uk_code_date_country_province_city"`
	Country   string    `gorm:"column:country;type:varchar(80);not null;default:Unknown;uniqueIndex:uk_code_date_country_province_city"`
	Province  string    `gorm:"column:province;type:varchar(50);not null;uniqueIndex:uk_code_date_country_province_city"`
	City      string    `gorm:"column:city;type:varchar(50);not null;uniqueIndex:uk_code_date_country_province_city"`
	Clicks    uint      `gorm:"column:clicks;not null"`
}

func (StatsRegionDaily) TableName() string {
	return "stats_region_daily"
}

// StatsDeviceDaily ��Ӧ���ݿ��е� stats_device_daily ���
type StatsDeviceDaily struct {
	ID         uint      `gorm:"primarykey"`
	ShortCode  string    `gorm:"column:short_code;type:varchar(20);not null;uniqueIndex:uk_code_date_device_os_browser"`
	Date       time.Time `gorm:"column:date;type:date;not null;uniqueIndex:uk_code_date_device_os_browser"`
	DeviceType string    `gorm:"column:device_type;type:varchar(20);not null;uniqueIndex:uk_code_date_device_os_browser"`
	OsVersion  string    `gorm:"column:os_version;type:varchar(50);not null;uniqueIndex:uk_code_date_device_os_browser"`
	Browser    string    `gorm:"column:browser;type:varchar(50);not null;uniqueIndex:uk_code_date_device_os_browser"`
	Clicks     uint      `gorm:"column:clicks;not null"`
}

func (StatsDeviceDaily) TableName() string {
	return "stats_device_daily"
}
