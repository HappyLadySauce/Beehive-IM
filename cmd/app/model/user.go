package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
    ID           uint           `gorm:"primaryKey" json:"id"`
    Username     string         `gorm:"type:varchar(50);unique;not null" json:"username"`
    Email        string         `gorm:"type:varchar(255);unique;not null" json:"email"`
    PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
    Status       string         `gorm:"type:varchar(20);default:'active'" json:"status"`
    LastLoginAt  time.Time      `json:"last_login_at"`
    CreatedAt    time.Time      `json:"created_at"`
    UpdatedAt    time.Time      `json:"updated_at"`
    DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
    return "users"
}

