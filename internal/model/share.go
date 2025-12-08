package model

import (

    "gorm.io/gorm"
)

type Share struct {
    gorm.Model
    ShortCode  string    `gorm:"column:short_code;not null" json:"shortCode"`         // 关联短码
    ShareTitle string    `gorm:"column:share_title;default:NULL" json:"shareTitle"`   // 分享标题
    ShareDesc  string    `gorm:"column:share_desc;default:NULL" json:"shareDesc"`     // 分享描述
    ShareImage string    `gorm:"column:share_image;default:NULL" json:"shareImage"`   // 分享封面图URL
}

func (Share) TableName() string {
    return "shares"
}