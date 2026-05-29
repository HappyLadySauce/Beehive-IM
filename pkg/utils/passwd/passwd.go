package passwd

import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)

// 默认 bcrypt 成本因子（10 是常用值，可根据性能调整）
const defaultCost = 10

// HashPassword 将明文密码加密为 bcrypt 哈希串
func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), defaultCost)
    if err != nil {
        return "", fmt.Errorf("密码加密失败: %w", err)
    }
    return string(hash), nil
}

// CheckPassword 验证明文密码是否与哈希串匹配
func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}