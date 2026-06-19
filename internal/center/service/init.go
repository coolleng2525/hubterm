package service

import (
	"crypto/rand"
	"encoding/hex"
	stdlog "log"
	"os"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var svcLog = log.New("service")

// EnsureAdminExists creates the built-in admin and treats ADMIN_PASSWORD as
// the source of truth when it is explicitly configured.
func EnsureAdminExists() {
	db := model.GetDB()
	var admin model.User
	err := db.Where("username = ?", "admin").First(&admin).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		svcLog.Error("failed to query admin user", log.Err(err))
		return
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if err == gorm.ErrRecordNotFound {
		if password == "" {
			// Generate a random 16-char hex password
			b := make([]byte, 8)
			if _, err := rand.Read(b); err != nil {
				stdlog.Fatalf("failed to generate random admin password: %v", err)
			}
			password = hex.EncodeToString(b)
			svcLog.Info("generated random admin password - SAVE THIS",
				log.String("username", "admin"),
				log.String("password", password),
			)
		}

		hash, err := HashPassword(password)
		if err != nil {
			svcLog.Error("failed to hash admin password", log.Err(err))
			return
		}

		if err := db.Create(&model.User{
			Username:     "admin",
			PasswordHash: hash,
			Role:         "admin",
		}).Error; err != nil {
			svcLog.Error("failed to create admin user", log.Err(err))
		}
		return
	}

	if password == "" {
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) == nil && admin.Role == "admin" {
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		svcLog.Error("failed to hash configured admin password", log.Err(err))
		return
	}
	if err := db.Model(&admin).Updates(map[string]interface{}{
		"password_hash": hash,
		"role":          "admin",
	}).Error; err != nil {
		svcLog.Error("failed to synchronize admin credentials", log.Err(err))
		return
	}
	svcLog.Info("admin credentials synchronized from environment", log.String("username", "admin"))
}
