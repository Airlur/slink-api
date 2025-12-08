package model

import (
    "time"
    "gorm.io/gorm"
)

type Tag struct {
    ID        uint           `gorm:"primarykey"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
    ShortCode string         `gorm:"column:short_code;not null" json:"shortCode"`   // 关联短码
    UserId    int64          `gorm:"column:user_id;not null" json:"userId"`         // 关联用户ID
    TagName   string         `gorm:"column:tag_name;not null" json:"tagName"`       // 标签名称（如“活动推广”）
}

func (Tag) TableName() string {
    return "tags"
}