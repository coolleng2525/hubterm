package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"github.com/coolleng2525/hubterm/internal/pkg/script"
)

// ScriptHandler handles script management API endpoints.
type ScriptHandler struct {
	DB     *gorm.DB
	Engine *script.Engine
}

var scriptLog = log.New("script_handler")

// NewScriptHandler creates a new ScriptHandler with the given database and script engine.
func NewScriptHandler(db *gorm.DB, engine *script.Engine) *ScriptHandler {
	return &ScriptHandler{
		DB:     db,
		Engine: engine,
	}
}

// Create handles POST /api/scripts — upload a new script.
// Request body: {"name": "...", "description": "...", "language": "python", "source": "...", "params": [...], "timeout": 30}
func (h *ScriptHandler) Create(c *gin.Context) {
	var req struct {
		Name        string         `json:"name" binding:"required"`
		Description string         `json:"description"`
		Language    string         `json:"language"`
		Source      string         `json:"source" binding:"required"`
		Params      []script.Param `json:"params"`
		Timeout     int            `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default language to python.
	if req.Language == "" {
		req.Language = "python"
	}

	// Validate script syntax for Python scripts.
	if req.Language == "python" {
		if err := h.Engine.Validate(req.Source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "syntax error: " + err.Error()})
			return
		}
	}

	// Serialize params to JSON.
	paramsJSON := "[]"
	if len(req.Params) > 0 {
		b, err := json.Marshal(req.Params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize params"})
			return
		}
		paramsJSON = string(b)
	}

	username, _ := c.Get("username")

	scriptModel := model.Script{
		ScriptID:    uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Language:    req.Language,
		Source:      req.Source,
		Params:      paramsJSON,
		Timeout:     req.Timeout,
		CreatedBy:   username.(string),
	}

	if err := h.DB.Create(&scriptModel).Error; err != nil {
		scriptLog.Error("failed to create script", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	scriptLog.Info("script created",
		log.String("script_id", scriptModel.ScriptID),
		log.String("name", scriptModel.Name),
	)

	c.JSON(http.StatusCreated, scriptModel)
}

// Execute handles POST /api/scripts/:id/execute — execute a script locally on the center.
func (h *ScriptHandler) Execute(c *gin.Context) {
	id := c.Param("id")

	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	var req struct {
		Params map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Params = nil
	}

	// Parse stored params.
	scriptParams := parseScriptParams(scriptModel.Params)

	scriptDef := &script.Script{
		ID:       scriptModel.ScriptID,
		Name:     scriptModel.Name,
		Language: scriptModel.Language,
		Source:   scriptModel.Source,
		Params:   scriptParams,
		Timeout:  scriptModel.Timeout,
	}

	result, err := h.Engine.Execute(scriptDef, req.Params)
	if err != nil {
		scriptLog.Warn("script execution error",
			log.String("script_id", scriptModel.ScriptID),
			log.Err(err),
		)
	}

	// Store result in database.
	status := "completed"
	if result.ExitCode != 0 {
		status = "failed"
	}
	if err != nil && result.ExitCode == -1 {
		status = "failed"
	}

	startedAt := time.UnixMilli(result.StartedAt)
	completedAt := time.UnixMilli(result.CompletedAt)

	resultModel := model.ScriptResult{
		ScriptID:    scriptModel.ScriptID,
		NodeID:      "",
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		ExitCode:    result.ExitCode,
		Duration:    result.Duration,
		Status:      status,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}
	if err := h.DB.Create(&resultModel).Error; err != nil {
		scriptLog.Error("failed to store script result", log.Err(err))
	}

	scriptLog.Info("script executed",
		log.String("script_id", scriptModel.ScriptID),
		log.Int("exit_code", result.ExitCode),
		log.Int("duration_ms", int(result.Duration)),
	)

	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}

// ExecuteOnNode handles POST /api/scripts/:id/execute-on-node/:node_id — execute a script on a remote node.
func (h *ScriptHandler) ExecuteOnNode(c *gin.Context) {
	id := c.Param("id")
	nodeID := c.Param("node_id")

	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	var req struct {
		Params map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Params = nil
	}

	scriptParams := parseScriptParams(scriptModel.Params)

	scriptDef := &script.Script{
		ID:       scriptModel.ScriptID,
		Name:     scriptModel.Name,
		Language: scriptModel.Language,
		Source:   scriptModel.Source,
		Params:   scriptParams,
		Timeout:  scriptModel.Timeout,
	}

	result, err := h.Engine.ExecuteOnNode(scriptDef, req.Params, nodeID)
	if err != nil {
		scriptLog.Error("remote execution not available",
			log.String("script_id", scriptModel.ScriptID),
			log.String("node_id", nodeID),
			log.Err(err),
		)
		c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}

// List handles GET /api/scripts — list all scripts.
func (h *ScriptHandler) List(c *gin.Context) {
	var scripts []model.Script
	if err := h.DB.Order("updated_at desc").Find(&scripts).Error; err != nil {
		scriptLog.Error("failed to list scripts", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, scripts)
}

// Get handles GET /api/scripts/:id — get script details.
func (h *ScriptHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}
	c.JSON(http.StatusOK, scriptModel)
}

// Delete handles DELETE /api/scripts/:id — delete a script.
func (h *ScriptHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}
	if err := h.DB.Delete(&scriptModel).Error; err != nil {
		scriptLog.Error("failed to delete script", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	scriptLog.Info("script deleted",
		log.String("script_id", scriptModel.ScriptID),
		log.String("name", scriptModel.Name),
	)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Update handles PUT /api/scripts/:id — update an existing script.
func (h *ScriptHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	var req struct {
		Name        string         `json:"name" binding:"required"`
		Description string         `json:"description"`
		Language    string         `json:"language"`
		Source      string         `json:"source" binding:"required"`
		Params      []script.Param `json:"params"`
		Timeout     int            `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Language == "" {
		req.Language = "python"
	}
	if req.Language == "python" {
		if err := h.Engine.Validate(req.Source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "syntax error: " + err.Error()})
			return
		}
	}

	paramsJSON := "[]"
	if len(req.Params) > 0 {
		b, err := json.Marshal(req.Params)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid params"})
			return
		}
		paramsJSON = string(b)
	}

	scriptModel.Name = req.Name
	scriptModel.Description = req.Description
	scriptModel.Language = req.Language
	scriptModel.Source = req.Source
	scriptModel.Params = paramsJSON
	scriptModel.Timeout = req.Timeout

	if err := h.DB.Save(&scriptModel).Error; err != nil {
		scriptLog.Error("failed to update script", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	scriptLog.Info("script updated", log.String("script_id", scriptModel.ScriptID))
	c.JSON(http.StatusOK, scriptModel)
}

// Results handles GET /api/scripts/:id/results — list execution history for a script.
func (h *ScriptHandler) Results(c *gin.Context) {
	id := c.Param("id")
	var results []model.ScriptResult
	if err := h.DB.Where("script_id = ?", id).Order("created_at desc").Find(&results).Error; err != nil {
		scriptLog.Error("failed to list script results", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, results)
}

// presetScript is the per-script entry in an import/export bundle.
type presetScript struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Language    string         `json:"language"`
	Source      string         `json:"source,omitempty"`
	SourceFile  string         `json:"source_file,omitempty"`
	Params      []script.Param `json:"params,omitempty"`
	Timeout     int            `json:"timeout,omitempty"`
}

// presetBundle is the top-level import/export JSON document.
type presetBundle struct {
	Version    string         `json:"version"`
	ExportedAt string         `json:"exported_at"`
	Scripts    []presetScript `json:"scripts"`
	Overwrite  bool           `json:"overwrite,omitempty"`
}

type presetPackageInfo struct {
	PackageVersion string `json:"package_version"`
	BundleVersion  string `json:"bundle_version"`
	CreatedAt      string `json:"created_at"`
	Format         string `json:"format"`
	Encrypted      bool   `json:"encrypted"`
	Cipher         string `json:"cipher,omitempty"`
	KDF            string `json:"kdf,omitempty"`
	Iterations     int    `json:"iterations,omitempty"`
	Salt           string `json:"salt,omitempty"`
	IV             string `json:"iv,omitempty"`
	PayloadFile    string `json:"payload_file,omitempty"`
	PayloadFormat  string `json:"payload_format,omitempty"`
}

var unsafeBundleNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Export handles GET /api/scripts/export.
// By default it returns a JSON preset bundle. With ?format=tar or ?format=tar.gz
// it returns a tar package containing manifest.json and script files.
func (h *ScriptHandler) Export(c *gin.Context) {
	var scripts []model.Script
	if err := h.DB.Order("name asc").Find(&scripts).Error; err != nil {
		scriptLog.Error("export: failed to query scripts", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	format := strings.ToLower(c.Query("format"))
	if format == "tar" || format == "tar.gz" || format == "tgz" {
		password := c.GetHeader("X-HubTerm-Export-Password")
		data, filename, err := buildScriptTarBundle(scripts, format != "tar", password)
		if err != nil {
			scriptLog.Error("export: failed to build tar bundle", log.Err(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build export package"})
			return
		}
		contentType := "application/x-tar"
		if strings.HasSuffix(filename, ".gz") {
			contentType = "application/gzip"
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Data(http.StatusOK, contentType, data)
		return
	}

	bundle := buildInlinePresetBundle(scripts)
	scriptLog.Info("scripts exported", log.Int("count", len(bundle.Scripts)))
	c.JSON(http.StatusOK, bundle)
}

// Import handles POST /api/scripts/import — bulk-upsert scripts from a JSON
// preset bundle, uploaded JSON file, tar package, or tar.gz package.
// Scripts are matched by name: existing ones are updated, new ones are created.
func (h *ScriptHandler) Import(c *gin.Context) {
	bundle, err := h.readImportBundle(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.importPresetBundle(bundle, bundle.Overwrite)
	c.JSON(http.StatusOK, result)
}

type scriptImportResult struct {
	Imported int `json:"imported"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
}

func (h *ScriptHandler) importPresetBundle(bundle presetBundle, overwriteExisting bool) scriptImportResult {
	imported := 0
	updated := 0
	skipped := 0

	for _, ps := range bundle.Scripts {
		if ps.Name == "" {
			skipped++
			continue
		}
		lang := ps.Language
		if lang == "" {
			lang = "text"
		}
		paramsJSON := "[]"
		if len(ps.Params) > 0 {
			if data, err := json.Marshal(ps.Params); err == nil {
				paramsJSON = string(data)
			}
		}
		timeout := ps.Timeout
		if timeout == 0 {
			timeout = 30
		}

		var existing model.Script
		err := h.DB.Where("name = ?", ps.Name).First(&existing).Error
		if err != nil {
			newScript := model.Script{
				ScriptID:    uuid.New().String(),
				Name:        ps.Name,
				Description: ps.Description,
				Language:    lang,
				Source:      ps.Source,
				Params:      paramsJSON,
				Timeout:     timeout,
			}
			if dbErr := h.DB.Create(&newScript).Error; dbErr != nil {
				scriptLog.Error("import: failed to create script",
					log.String("name", ps.Name),
					log.Err(dbErr),
				)
				continue
			}
			imported++
		} else {
			if !overwriteExisting {
				skipped++
				continue
			}
			existing.Description = ps.Description
			existing.Language = lang
			existing.Source = ps.Source
			existing.Params = paramsJSON
			existing.Timeout = timeout
			if dbErr := h.DB.Save(&existing).Error; dbErr != nil {
				scriptLog.Error("import: failed to update script",
					log.String("name", ps.Name),
					log.Err(dbErr),
				)
				continue
			}
			updated++
		}
	}

	scriptLog.Info("scripts imported",
		log.Int("imported", imported),
		log.Int("updated", updated),
		log.Int("skipped", skipped),
	)
	return scriptImportResult{Imported: imported, Updated: updated, Skipped: skipped}
}

// parseScriptParams parses the JSON params string from the database into []script.Param.
func parseScriptParams(paramsJSON string) []script.Param {
	if paramsJSON == "" || paramsJSON == "[]" {
		return nil
	}
	var params []script.Param
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		scriptLog.Warn("failed to parse script params", log.Err(err))
		return nil
	}
	return params
}

func (h *ScriptHandler) readImportBundle(c *gin.Context) (presetBundle, error) {
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		file, err := c.FormFile("file")
		if err != nil {
			return presetBundle{}, fmt.Errorf("missing import file")
		}
		src, err := file.Open()
		if err != nil {
			return presetBundle{}, fmt.Errorf("failed to open import file")
		}
		defer src.Close()
		data, err := io.ReadAll(io.LimitReader(src, 50*1024*1024))
		if err != nil {
			return presetBundle{}, fmt.Errorf("failed to read import file")
		}
		return parsePresetBundleFile(file.Filename, data, c.PostForm("password"))
	}

	var bundle presetBundle
	if err := c.ShouldBindJSON(&bundle); err != nil {
		return presetBundle{}, err
	}
	if err := hydrateBundleSources(&bundle, nil); err != nil {
		return presetBundle{}, err
	}
	return bundle, nil
}

func buildInlinePresetBundle(scripts []model.Script) presetBundle {
	bundle := presetBundle{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Scripts:    make([]presetScript, 0, len(scripts)),
	}
	for _, s := range scripts {
		bundle.Scripts = append(bundle.Scripts, presetScript{
			Name:        s.Name,
			Description: s.Description,
			Language:    s.Language,
			Source:      s.Source,
			Params:      parseScriptParams(s.Params),
			Timeout:     s.Timeout,
		})
	}
	return bundle
}

func buildScriptTarBundle(scripts []model.Script, gzipOutput bool, password string) ([]byte, string, error) {
	payload, err := buildPlainScriptTarBundle(scripts)
	if err != nil {
		return nil, "", err
	}
	ext := ".tar"
	if gzipOutput {
		ext = ".tar.gz"
	}
	nameSuffix := ""
	if password != "" {
		nameSuffix = "-enc"
		payload, err = encryptPresetPayload(payload, password, "tar")
		if err != nil {
			return nil, "", err
		}
	}
	data, err := wrapTarPayload(payload, gzipOutput)
	if err != nil {
		return nil, "", err
	}
	return data, "hubterm-presets-" + time.Now().UTC().Format("2006-01-02") + nameSuffix + ext, nil
}

func buildPlainScriptTarBundle(scripts []model.Script) ([]byte, error) {
	bundle := presetBundle{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Scripts:    make([]presetScript, 0, len(scripts)),
	}
	files := map[string]string{}
	used := map[string]int{}
	for _, s := range scripts {
		filename := scriptBundleFilename(s.Name, s.Language, used)
		files[filename] = s.Source
		bundle.Scripts = append(bundle.Scripts, presetScript{
			Name:        s.Name,
			Description: s.Description,
			Language:    s.Language,
			SourceFile:  filename,
			Params:      parseScriptParams(s.Params),
			Timeout:     s.Timeout,
		})
	}

	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	info := presetPackageInfo{
		PackageVersion: "1.0",
		BundleVersion:  bundle.Version,
		CreatedAt:      bundle.ExportedAt,
		Format:         "hubterm-presets",
		Encrypted:      false,
	}
	infoJSON, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeTarFile(tw, "hubterm-package.json", infoJSON); err != nil {
		return nil, err
	}
	manifest, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeTarFile(tw, "manifest.json", manifest); err != nil {
		return nil, err
	}
	for name, source := range files {
		if err := writeTarFile(tw, name, []byte(source)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func wrapTarPayload(payload []byte, gzipOutput bool) ([]byte, error) {
	if !gzipOutput {
		return payload, nil
	}
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	if _, err := gz.Write(payload); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func encryptPresetPayload(payload []byte, password, payloadFormat string) ([]byte, error) {
	salt := make([]byte, 16)
	iv := make([]byte, 12)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	key := pbkdf2.Key([]byte(password), salt, 120000, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, iv, payload, nil)
	info := presetPackageInfo{
		PackageVersion: "1.0",
		BundleVersion:  "1.0",
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		Format:         "hubterm-presets",
		Encrypted:      true,
		Cipher:         "AES-256-GCM",
		KDF:            "PBKDF2-SHA256",
		Iterations:     120000,
		Salt:           base64.StdEncoding.EncodeToString(salt),
		IV:             base64.StdEncoding.EncodeToString(iv),
		PayloadFile:    "payload.enc",
		PayloadFormat:  payloadFormat,
	}
	infoJSON, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	if err := writeTarFile(tw, "hubterm-package.json", infoJSON); err != nil {
		return nil, err
	}
	if err := writeTarFile(tw, "payload.enc", ciphertext); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func decryptPresetPayload(info presetPackageInfo, files map[string][]byte, password string) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("encrypted package requires password")
	}
	if info.Cipher != "AES-256-GCM" || info.KDF != "PBKDF2-SHA256" {
		return nil, fmt.Errorf("unsupported encrypted package")
	}
	salt, err := base64.StdEncoding.DecodeString(info.Salt)
	if err != nil {
		return nil, fmt.Errorf("invalid encrypted package salt")
	}
	iv, err := base64.StdEncoding.DecodeString(info.IV)
	if err != nil {
		return nil, fmt.Errorf("invalid encrypted package iv")
	}
	payloadFile := info.PayloadFile
	if payloadFile == "" {
		payloadFile = "payload.enc"
	}
	ciphertext, ok := files[payloadFile]
	if !ok {
		return nil, fmt.Errorf("encrypted package missing payload")
	}
	iterations := info.Iterations
	if iterations <= 0 {
		iterations = 120000
	}
	key := pbkdf2.Key([]byte(password), salt, iterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt encrypted package: %w", err)
	}
	return plain, nil
}
func writeTarFile(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Mode:    0600,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func parsePresetBundleFile(filename string, data []byte, password string) (presetBundle, error) {
	name := strings.ToLower(filename)
	if strings.HasSuffix(name, ".tar") || strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
		return parsePresetTarBundle(data, strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".tgz"), password)
	}
	var bundle presetBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return presetBundle{}, fmt.Errorf("invalid JSON bundle: %w", err)
	}
	if err := hydrateBundleSources(&bundle, nil); err != nil {
		return presetBundle{}, err
	}
	return bundle, nil
}

func parsePresetTarBundle(data []byte, gzipped bool, password string) (presetBundle, error) {
	reader := io.Reader(bytes.NewReader(data))
	var gz *gzip.Reader
	var err error
	if gzipped {
		gz, err = gzip.NewReader(reader)
		if err != nil {
			return presetBundle{}, fmt.Errorf("invalid gzip package: %w", err)
		}
		defer gz.Close()
		reader = gz
	}
	tr := tar.NewReader(reader)
	files := map[string][]byte{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return presetBundle{}, fmt.Errorf("invalid tar package: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		cleanName, err := cleanBundlePath(header.Name)
		if err != nil {
			return presetBundle{}, err
		}
		content, err := io.ReadAll(io.LimitReader(tr, 10*1024*1024))
		if err != nil {
			return presetBundle{}, fmt.Errorf("failed to read %s: %w", cleanName, err)
		}
		files[cleanName] = content
	}
	if infoData, ok := files["hubterm-package.json"]; ok {
		var info presetPackageInfo
		if err := json.Unmarshal(infoData, &info); err != nil {
			return presetBundle{}, fmt.Errorf("invalid hubterm-package.json: %w", err)
		}
		if info.Encrypted {
			payload, err := decryptPresetPayload(info, files, password)
			if err != nil {
				return presetBundle{}, err
			}
			return parsePresetTarBundle(payload, info.PayloadFormat == "tar.gz" || info.PayloadFormat == "tgz", "")
		}
	}
	manifest, ok := files["manifest.json"]
	if !ok {
		return presetBundle{}, fmt.Errorf("package missing manifest.json")
	}
	var bundle presetBundle
	if err := json.Unmarshal(manifest, &bundle); err != nil {
		return presetBundle{}, fmt.Errorf("invalid manifest.json: %w", err)
	}
	if err := hydrateBundleSources(&bundle, files); err != nil {
		return presetBundle{}, err
	}
	return bundle, nil
}

func hydrateBundleSources(bundle *presetBundle, files map[string][]byte) error {
	for i := range bundle.Scripts {
		ps := &bundle.Scripts[i]
		if ps.Language == "" {
			ps.Language = inferLanguage(ps.SourceFile)
		}
		if ps.Source == "" && ps.SourceFile != "" {
			cleanName, err := cleanBundlePath(ps.SourceFile)
			if err != nil {
				return err
			}
			if files == nil {
				return fmt.Errorf("source_file requires a package file: %s", cleanName)
			}
			content, ok := files[cleanName]
			if !ok {
				return fmt.Errorf("source file not found in package: %s", cleanName)
			}
			ps.Source = string(content)
		}
		if ps.Language == "" {
			ps.Language = "text"
		}
	}
	return nil
}

func cleanBundlePath(name string) (string, error) {
	cleaned := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("invalid package path: %s", name)
	}
	return cleaned, nil
}

func scriptBundleFilename(name, language string, used map[string]int) string {
	base := strings.Trim(unsafeBundleNameChars.ReplaceAllString(name, "-"), ".-")
	if base == "" {
		base = "script"
	}
	ext := ".txt"
	switch language {
	case "python":
		ext = ".py"
	case "shell":
		ext = ".sh"
	}
	filename := "files/" + base + ext
	used[filename]++
	if used[filename] > 1 {
		filename = fmt.Sprintf("files/%s-%d%s", base, used[filename], ext)
	}
	return filename
}

func inferLanguage(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".py":
		return "python"
	case ".sh", ".bash":
		return "shell"
	default:
		return "text"
	}
}

// LoadPresetsFromDir reads preset bundles from dir and upserts scripts into the
// database. It supports:
//   - *.json inline bundles
//   - *.tar / *.tar.gz packages with manifest.json
//   - subdirectories that contain manifest.json plus referenced files
//
// Called once on startup when cfg.Presets.Dir is configured.
func (h *ScriptHandler) LoadPresetsFromDir(dir string) {
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		scriptLog.Warn("presets dir not readable", log.String("dir", dir), log.Err(err))
		return
	}

	total, skipped := 0, 0
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		bundle, err := readPresetPath(path, entry)
		if err != nil {
			scriptLog.Warn("failed to read preset bundle", log.String("path", path), log.Err(err))
			continue
		}
		result := h.importPresetBundle(bundle, false)
		total += result.Imported
		skipped += result.Skipped + result.Updated
	}
	scriptLog.Info("presets loaded from dir",
		log.String("dir", dir),
		log.Int("created", total),
		log.Int("skipped_existing", skipped),
	)
}

func readPresetPath(path string, entry os.DirEntry) (presetBundle, error) {
	if entry.IsDir() {
		return readPresetDirectory(path)
	}
	name := strings.ToLower(entry.Name())
	if !(strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".tar") || strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz")) {
		return presetBundle{}, fmt.Errorf("unsupported preset file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return presetBundle{}, err
	}
	return parsePresetBundleFile(entry.Name(), data, "")
}

func readPresetDirectory(dir string) (presetBundle, error) {
	manifestPath := filepath.Join(dir, "manifest.json")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return presetBundle{}, err
	}
	var bundle presetBundle
	if err := json.Unmarshal(manifest, &bundle); err != nil {
		return presetBundle{}, fmt.Errorf("invalid manifest.json: %w", err)
	}
	files := map[string][]byte{}
	for i := range bundle.Scripts {
		sourceFile := bundle.Scripts[i].SourceFile
		if sourceFile == "" {
			continue
		}
		cleanName, err := cleanBundlePath(sourceFile)
		if err != nil {
			return presetBundle{}, err
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(cleanName)))
		if err != nil {
			return presetBundle{}, fmt.Errorf("read source file %s: %w", cleanName, err)
		}
		files[cleanName] = data
	}
	if err := hydrateBundleSources(&bundle, files); err != nil {
		return presetBundle{}, err
	}
	return bundle, nil
}
