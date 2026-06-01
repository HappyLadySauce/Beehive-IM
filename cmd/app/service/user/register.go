package user

import (
	"context"

	"github.com/HappyLadySauce/Beehive-IM/cmd/app/model"
	"github.com/HappyLadySauce/Beehive-IM/cmd/app/types/api/v1"
)

// Register handles user registration logic, including input validation, password hashing, and database storage.
// Register 处理用户注册逻辑，包括输入验证、密码哈希和数据库存储。
func (s *UserService) Register(ctx context.Context, req v1.RegisterRequest) (string, error) {
	username := req.Username
	email := req.Email
	password := req.Password

	s.DB.WithContext(ctx).Where("username = ?", username).Or("email = ?", email).First(&model.User{})

}

