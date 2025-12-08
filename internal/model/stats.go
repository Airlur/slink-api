package model

import "time"

// StatsDaily 对应数据库中的 stats_daily 表
type StatsDaily struct {
	ID        uint      `gorm:"primarykey"` 														  // 唯一标识主键
	ShortCode string    `gorm:"column:short_code;type:varchar(20);not null;uniqueIndex:uk_code_date"` // 关联短码
	Date      time.Time `gorm:"column:date;type:date;not null;uniqueIndex:uk_code_date"` 			  // 统计日期
	Clicks    uint      `gorm:"column:clicks;not null"` 											  // 当然点击量
}

func (StatsDaily) TableName() string {
	return "stats_daily"	// 按日点击量统计表
}

// StatsRegionDaily 对应数据库中的 stats_region_daily 表
type StatsRegionDaily struct {
	ID        uint      `gorm:"primarykey"` 															        	// 唯一标识主键
	ShortCode string    `gorm:"column:short_code;type:varchar(20);not null;uniqueIndex:uk_code_date_province_city"` // 关联短码
	Date      time.Time `gorm:"column:date;type:date;not null;uniqueIndex:uk_code_date_province_city"`              // 统计日期
	Province  string    `gorm:"column:province;type:varchar(50);not null;uniqueIndex:uk_code_date_province_city"`   // 省份
	City      string    `gorm:"column:city;type:varchar(50);not null;uniqueIndex:uk_code_date_province_city"`       // 城市
	Clicks    uint      `gorm:"column:clicks;not null"`																// 当日该地区点击量
}

func (StatsRegionDaily) TableName() string {
	return "stats_region_daily"	// 按地域每日点击量统计表
}


// StatsDeviceDaily 对应数据库中的 stats_device_daily 表
type StatsDeviceDaily struct {
	ID         uint      `gorm:"primarykey"`																			  // 唯一标识主键
	ShortCode  string    `gorm:"column:short_code;type:varchar(20);not null;uniqueIndex:uk_code_date_device_os_browser"`  // 关联短码
	Date       time.Time `gorm:"column:date;type:date;not null;uniqueIndex:uk_code_date_device_os_browser"`				  // 统计日期
	DeviceType string    `gorm:"column:device_type;type:varchar(20);not null;uniqueIndex:uk_code_date_device_os_browser"` //s 设备类型
	OsVersion  string    `gorm:"column:os_version;type:varchar(50);not null;uniqueIndex:uk_code_date_device_os_browser"`  // 操作系统
	Browser    string    `gorm:"column:browser;type:varchar(50);not null;uniqueIndex:uk_code_date_device_os_browser"`     // 浏览器
	Clicks     uint      `gorm:"column:clicks;not null"`																  // 当日该设备组合点击量
}

func (StatsDeviceDaily) TableName() string {
	return "stats_device_daily"	// 按设备每日点击量统计表
}