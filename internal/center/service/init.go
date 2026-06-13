package service

import (
	"crypto/rand"
	"encoding/hex"
	stdlog "log"
	"os"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

var svcLog = log.New("service")

// FIXED: Admin password read from ADMIN_PASSWORD env var, or generated randomly
func EnsureAdminExists() {
	db := model.GetDB()
	var count int64
	if err := db.Model(&model.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		svcLog.Error("failed to count admin users", log.Err(err))
		return
	}
	if count == 0 {
		password := os.Getenv("ADMIN_PASSWORD")
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
	}
}
