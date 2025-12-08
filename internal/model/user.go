package model

import (
	"time"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model         // 包含 ID, CreatedAt, UpdatedAt, DeletedAt
	Username    string 		`gorm:"unique;not null;size:64" json:"username"` // 用户名
	Password    string 		`gorm:"not null;size:128" json:"-"`              // 密码
	Nickname    string 		`gorm:"size:64" json:"nickname"`                 // 昵称
	Avatar      string 		`gorm:"size:255" json:"avatar"`                  // 头像URL
	Phone       string 		`gorm:"size:20" json:"phone"`                    // 手机号
	Email       string 		`gorm:"size:128" json:"email"`                   // 邮箱
	Status      int    		`gorm:"default:1" json:"status"`                 // 状态 1:正常 2:禁用
	LastLoginAt *time.Time  `gorm:"" json:"last_login_at"`               	 // 最后登录时间
	Role        int    		`gorm:"default:1" json:"role"`                   // 角色 1:普通用户 2:管理员
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// 用户状态常量
const (
	UserStatusPending      = 0 // 待验证 (用于邮件激活等场景)
	UserStatusNormal       = 1 // 正常
	UserStatusBanned       = 2 // 禁用 (由管理员操作，性质严重)
	UserStatusLocked       = 3 // 锁定 (通常是系统自动行为，有时间限制)
	UserStatusCancellation = 4 // 已注销 (用户主动操作)
)

// 用户角色常量
const (
	UserRoleNormal = 1 // 普通用户
	UserRoleAdmin  = 2 // 管理员
)
