package service

import (
	"testing"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func setupAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	model.DB = db
	return db
}

func TestEnsureAdminExistsSynchronizesConfiguredPassword(t *testing.T) {
	db := setupAdminTestDB(t)
	oldHash, _ := HashPassword("old-password")
	if err := db.Create(&model.User{Username: "admin", PasswordHash: oldHash, Role: "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_PASSWORD", "new-password")

	EnsureAdminExists()

	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte("new-password")); err != nil {
		t.Fatal("configured password was not synchronized")
	}
}

func TestEnsureAdminExistsKeepsPasswordWithoutConfiguration(t *testing.T) {
	db := setupAdminTestDB(t)
	oldHash, _ := HashPassword("old-password")
	if err := db.Create(&model.User{Username: "admin", PasswordHash: oldHash, Role: "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_PASSWORD", "")

	EnsureAdminExists()

	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte("old-password")); err != nil {
		t.Fatal("existing password changed without ADMIN_PASSWORD")
	}
}
