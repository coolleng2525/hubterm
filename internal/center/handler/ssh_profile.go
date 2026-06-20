package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/securestore"
)

type SSHProfileHandler struct{ DB *gorm.DB }

type sshProfileRequest struct {
	Name       string `json:"name"`
	NodeID     string `json:"node_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	AuthType   string `json:"auth_type"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase"`
}

type sshProfileResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	NodeID      string `json:"node_id"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	AuthType    string `json:"auth_type"`
	HasPassword bool   `json:"has_password"`
	HasKey      bool   `json:"has_private_key"`
}

func profileResponse(p model.SSHProfile) sshProfileResponse {
	return sshProfileResponse{p.ID, p.Name, p.NodeID, p.Host, p.Port, p.Username, p.AuthType, p.EncryptedPassword != "", p.EncryptedPrivateKey != ""}
}

func currentUserID(c *gin.Context) uint {
	value, _ := c.Get("user_id")
	id, _ := value.(uint)
	return id
}

func (h *SSHProfileHandler) List(c *gin.Context) {
	var profiles []model.SSHProfile
	query := h.DB.Where("user_id = ?", currentUserID(c)).Order("name")
	if nodeID := c.Query("node_id"); nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if err := query.Find(&profiles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list SSH profiles"})
		return
	}
	result := make([]sshProfileResponse, len(profiles))
	for i, profile := range profiles {
		result[i] = profileResponse(profile)
	}
	c.JSON(http.StatusOK, result)
}

func validateSSHProfile(req sshProfileRequest, existing *model.SSHProfile) string {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Username) == "" {
		return "name, host and username are required"
	}
	if req.Port < 1 || req.Port > 65535 {
		return "invalid SSH port"
	}
	if req.AuthType != "password" && req.AuthType != "key" {
		return "auth_type must be password or key"
	}
	if req.AuthType == "password" && req.Password == "" && (existing == nil || existing.EncryptedPassword == "") {
		return "password is required"
	}
	if req.AuthType == "key" && req.PrivateKey == "" && (existing == nil || existing.EncryptedPrivateKey == "") {
		return "private key is required"
	}
	return ""
}

func applySSHProfileRequest(profile *model.SSHProfile, req sshProfileRequest) error {
	profile.Name = strings.TrimSpace(req.Name)
	profile.NodeID = req.NodeID
	profile.Host = strings.TrimSpace(req.Host)
	profile.Port = req.Port
	profile.Username = strings.TrimSpace(req.Username)
	profile.AuthType = req.AuthType
	if req.AuthType == "password" {
		profile.EncryptedPrivateKey = ""
		profile.EncryptedPassphrase = ""
	} else {
		profile.EncryptedPassword = ""
	}
	var err error
	if req.Password != "" {
		profile.EncryptedPassword, err = securestore.Encrypt(req.Password)
		if err != nil {
			return err
		}
	}
	if req.PrivateKey != "" {
		profile.EncryptedPrivateKey, err = securestore.Encrypt(req.PrivateKey)
		if err != nil {
			return err
		}
		if req.Passphrase == "" {
			profile.EncryptedPassphrase = ""
		}
	}
	if req.Passphrase != "" {
		profile.EncryptedPassphrase, err = securestore.Encrypt(req.Passphrase)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *SSHProfileHandler) Create(c *gin.Context) {
	var req sshProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if message := validateSSHProfile(req, nil); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	profile := model.SSHProfile{UserID: currentUserID(c)}
	if err := applySSHProfileRequest(&profile, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
		return
	}
	if err := h.DB.Create(&profile).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "profile name already exists"})
		return
	}
	c.JSON(http.StatusCreated, profileResponse(profile))
}

func (h *SSHProfileHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile id"})
		return
	}
	var profile model.SSHProfile
	if err := h.DB.Where("id = ? AND user_id = ?", id, currentUserID(c)).First(&profile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}
	var req sshProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if message := validateSSHProfile(req, &profile); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	if err := applySSHProfileRequest(&profile, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
		return
	}
	if err := h.DB.Save(&profile).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "profile name already exists"})
		return
	}
	c.JSON(http.StatusOK, profileResponse(profile))
}

func (h *SSHProfileHandler) Delete(c *gin.Context) {
	result := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), currentUserID(c)).Delete(&model.SSHProfile{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete profile"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func loadSSHProfile(db *gorm.DB, profileID, userID uint) (*model.SSHProfile, error) {
	var profile model.SSHProfile
	if err := db.Where("id = ? AND user_id = ?", profileID, userID).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}
