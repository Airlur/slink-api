package model

import "time"

// AccessLog ��Ӧ���ݿ��е� access_logs_YYYYMM ��ṹ��
// ����ʱ�ᶯָ̬���·ݷֱ��������ﲻ���� TableName��
type AccessLog struct {
	ID         uint      `gorm:"primarykey"`
	ShortCode  string    `gorm:"column:short_code;type:varchar(20);not null;index:idx_short_code_accessed_at,priority:1"`
	UserID     uint      `gorm:"column:user_id;not null"`
	IP         string    `gorm:"column:ip;type:varchar(45);not null"`
	UserAgent  string    `gorm:"column:user_agent;type:varchar(512)"`
	DeviceType string    `gorm:"column:device_type;type:varchar(20)"`
	OsVersion  string    `gorm:"column:os_version;type:varchar(50)"`
	Browser    string    `gorm:"column:browser;type:varchar(50)"`
	Country    string    `gorm:"column:country;type:varchar(80);index:idx_country_province_city,priority:1"`
	Province   string    `gorm:"column:province;type:varchar(50);index:idx_country_province_city,priority:2;index:idx_province_city,priority:1"`
	City       string    `gorm:"column:city;type:varchar(50);index:idx_country_province_city,priority:3;index:idx_province_city,priority:2"`
	Channel    string    `gorm:"column:channel;type:varchar(100)"`
	AccessedAt time.Time `gorm:"column:accessed_at;not null;index:idx_accessed_at;index:idx_short_code_accessed_at,priority:2"`
}
