package model

import "time"

// AccessLog 对应数据库中的 access_logs_YYYYMM 表结构
// 注意：没有 TableName() 方法，因为我们将动态指定表名
type AccessLog struct {
	ID         uint      `gorm:"primarykey"`                                                                                    // 日志唯一标识
	ShortCode  string    `gorm:"column:short_code;type:varchar(20);not null;index:idx_short_code_accessed_at,priority:1"`       // 关联的短码
	UserID     uint      `gorm:"column:user_id;not null"`                                                                       // 访问者用户ID（0代表未登录）
	IP         string    `gorm:"column:ip;type:varchar(45);not null"`                                                           // 访问IP地址
	UserAgent  string    `gorm:"column:user_agent;type:varchar(512)"`                                                           // 设备User-Agent
	DeviceType string    `gorm:"column:device_type;type:varchar(20)"`                                                           // 解析出的设备类型（PC/Mobile/Tablet/Other）
	OsVersion  string    `gorm:"column:os_version;type:varchar(50)"`                                                            // 解析出的操作系统
	Browser    string    `gorm:"column:browser;type:varchar(50)"`                                                               // 解析出的浏览器
	Province   string    `gorm:"column:province;type:varchar(50);index:idx_province_city,priority:1"` 							// 解析出的地理位置（省份）
	City       string    `gorm:"column:city;type:varchar(50);index:idx_province_city,priority:2"`     							// 解析出的地理位置（城市）
	Channel    string    `gorm:"column:channel;type:varchar(50)"`                           									// 访问来源渠道
	AccessedAt time.Time `gorm:"column:accessed_at;not null;index:idx_accessed_at;index:idx_short_code_accessed_at,priority:2"` // 访问时间
	// 注意：这张表我们采用物理删除或归档，因此不包含 DeletedAt 字段
}

// 按分表生命周期管理：
// 近期表（如 3 个月内）：保留在数据库，支持实时查询；
// 中期表（3 个月～1 年）：归档到冷存储（如 OSS），按需恢复查询；
// 过期表（1 年以上）：按合规要求直接物理删除（DROP TABLE）。