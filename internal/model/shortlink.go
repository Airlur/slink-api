package model

import (

	"time"

    "gorm.io/gorm"
)

type Shortlink struct {
    gorm.Model
    ShortCode      string    `gorm:"column:short_code;not null" json:"shortCode"`               // 短码（Base62，区分大小写）
    OriginalUrl    string    `gorm:"column:original_url;not null" json:"originalUrl"`           // 原始长链接（URLEncode编码）
    OriginalUrlMd5 string    `gorm:"column:original_url_md5;not null" json:"originalUrlMd5"`    // 原始长链接的MD5摘要
    UserId         int64     `gorm:"column:user_id;not null" json:"userId"`                     // 关联用户ID（管理员为0）
    ExpireAt       *time.Time `gorm:"column:expire_at;default:NULL" json:"expireAt"`            // 过期时间（NULL=永久）
    LastWarnAt     *time.Time `gorm:"column:last_warn_at;default:NULL" json:"lastWarnAt"`       // 最近一次失效预警发送时间
    Status         int       `gorm:"column:status;not null;default:1" json:"status"`            // 状态（1=有效，0=失效）
    ClickCount     int64     `gorm:"column:click_count;not null;default:0" json:"clickCount"`   // 点击量统计
    IsHot          int       `gorm:"column:is_hot;default:0" json:"isHot"`                      // 是否为热点短码（1=是，日访问≥1000）
    IsCustom       int       `gorm:"column:is_custom;default:0" json:"isCustom"`                // 是否为自定义短码（1=是）

    Tags  []Tag  `gorm:"foreignKey:ShortCode;references:ShortCode"` // 一对多关系
	Share *Share `gorm:"foreignKey:ShortCode;references:ShortCode"` // 一对一关系
}

func (Shortlink) TableName() string {
    return "shortlinks"
}