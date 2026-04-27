package main

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/storage"
)

type migrationSummary struct {
	scanned int
	updated int
	copied  int
	skipped int
	failed  int
}

type migrationApp struct {
	ctx        context.Context
	cfg        *config.Config
	db         database.DB
	provider   storage.StorageProvider
	bucket     string
	workDir    string
	apply      bool
	copiedKeys map[string]bool
	summaries  map[string]*migrationSummary
}

type plannedMigration struct {
	srcKey      string
	dstKey      string
	targetURL   string
	needsCopy   bool
	needsUpdate bool
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	apply := flag.Bool("apply", false, "copy objects and update database references")
	flag.Parse()

	_ = godotenv.Load()

	cfg := config.Load()
	workDir, _ := os.Getwd()
	db, err := database.InitDB(cfg.Database)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer db.Close()

	if err := database.RunAllMigrations(db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	provider, err := initStorageProvider(cfg, workDir)
	if err != nil {
		log.Fatalf("init storage provider: %v", err)
	}

	app := &migrationApp{
		ctx:        context.Background(),
		cfg:        cfg,
		db:         db,
		provider:   provider,
		bucket:     configuredStorageBucket(cfg),
		workDir:    workDir,
		apply:      *apply,
		copiedKeys: make(map[string]bool),
		summaries:  make(map[string]*migrationSummary),
	}

	if err := app.run(); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
}

func (a *migrationApp) run() error {
	log.Printf("storage layout migration started: mode=%s", ternary(a.apply, "apply", "dry-run"))

	steps := []struct {
		name string
		fn   func() error
	}{
		{name: "users.avatar", fn: a.migrateUserAvatars},
		{name: "appeals.evidence", fn: a.migrateAppealEvidence},
		{name: "claim_materials", fn: a.migrateClaimMaterials},
		{name: "inspirations", fn: a.migrateInspirations},
		{name: "inspiration_materials", fn: a.migrateInspirationMaterials},
	}

	for _, step := range steps {
		log.Printf("running step: %s", step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}

	log.Println("storage layout migration summary:")
	totalFailed := 0
	for _, name := range []string{"users.avatar", "appeals.evidence", "claim_materials", "inspirations", "inspiration_materials"} {
		s := a.summary(name)
		totalFailed += s.failed
		log.Printf("  %-22s scanned=%d updated=%d copied=%d skipped=%d failed=%d", name, s.scanned, s.updated, s.copied, s.skipped, s.failed)
	}
	if totalFailed > 0 {
		return fmt.Errorf("completed with %d failed items", totalFailed)
	}
	return nil
}

func (a *migrationApp) migrateUserAvatars() error {
	ok, err := a.hasColumns("users", "id", "avatar")
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("skip users.avatar: required columns are missing")
		return nil
	}

	rows, err := a.db.Query(`SELECT id, avatar FROM users WHERE TRIM(COALESCE(avatar, '')) <> ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type userAvatar struct {
		id     int64
		avatar string
	}

	var items []userAvatar
	for rows.Next() {
		var item userAvatar
		if err := rows.Scan(&item.id, &item.avatar); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range items {
		s := a.summary("users.avatar")
		s.scanned++

		legacyKey := a.normalizeObjectKey(item.avatar)
		if !isManagedAssetKey(legacyKey) {
			s.skipped++
			continue
		}

		filename := legacyFilename(legacyKey, fmt.Sprintf("avatar-%d%s", item.id, legacyExt(legacyKey, ".jpg")))
		dstKey := storage.BuildUploadObjectKey("image", storage.BizTypeAvatar, strconv.FormatInt(item.id, 10), "", filepath.Ext(filename), filename, item.id)
		moved, err := a.applyMigration("users.avatar", item.avatar, dstKey)
		if err != nil {
			s.failed++
			log.Printf("users.avatar id=%d: %v", item.id, err)
			continue
		}
		if !moved.changed {
			s.skipped++
			continue
		}
		s.copied += boolToInt(moved.copied)
		if a.apply {
			if _, err := a.db.Exec(`UPDATE users SET avatar = ?, updated_at = ? WHERE id = ?`, moved.url, time.Now(), item.id); err != nil {
				s.failed++
				log.Printf("users.avatar id=%d update db: %v", item.id, err)
				continue
			}
		}
		s.updated++
	}

	return nil
}

func (a *migrationApp) migrateAppealEvidence() error {
	ok, err := a.hasColumns("appeals", "id", "user_id", "evidence")
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("skip appeals.evidence: required columns are missing")
		return nil
	}

	rows, err := a.db.Query(`SELECT id, user_id, evidence FROM appeals WHERE TRIM(COALESCE(evidence, '')) <> ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type appealEvidence struct {
		id       int64
		userID   int64
		evidence string
	}

	var items []appealEvidence
	for rows.Next() {
		var item appealEvidence
		if err := rows.Scan(&item.id, &item.userID, &item.evidence); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range items {
		s := a.summary("appeals.evidence")
		s.scanned++

		parts := strings.Split(item.evidence, ",")
		updatedParts := make([]string, 0, len(parts))
		rowChanged := false
		rowCopied := 0

		for idx, rawPart := range parts {
			rawPart = strings.TrimSpace(rawPart)
			if rawPart == "" {
				continue
			}
			legacyKey := a.normalizeObjectKey(rawPart)
			if !isManagedAssetKey(legacyKey) {
				updatedParts = append(updatedParts, rawPart)
				continue
			}
			ext := legacyExt(legacyKey, ".jpg")
			filename := fmt.Sprintf("appeal-%d-%d%s", item.id, idx+1, ext)
			dstKey := storage.BuildUploadObjectKey("image", storage.BizTypeAppealEvidence, strconv.FormatInt(item.userID, 10), "", ext, filename, item.userID)
			moved, err := a.applyMigration("appeals.evidence", rawPart, dstKey)
			if err != nil {
				s.failed++
				log.Printf("appeals.evidence id=%d item=%d: %v", item.id, idx+1, err)
				updatedParts = append(updatedParts, rawPart)
				continue
			}
			rowChanged = rowChanged || moved.changed
			rowCopied += boolToInt(moved.copied)
			updatedParts = append(updatedParts, moved.url)
		}

		if !rowChanged {
			s.skipped++
			continue
		}
		s.copied += rowCopied
		if a.apply {
			if _, err := a.db.Exec(`UPDATE appeals SET evidence = ? WHERE id = ?`, strings.Join(updatedParts, ","), item.id); err != nil {
				s.failed++
				log.Printf("appeals.evidence id=%d update db: %v", item.id, err)
				continue
			}
		}
		s.updated++
	}

	return nil
}

func (a *migrationApp) migrateClaimMaterials() error {
	ok, err := a.hasColumns("claim_materials", "id", "claim_id", "file_type", "file_path", "source_file_path", "processed_file_path", "thumbnail_path")
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("skip claim_materials: required columns are missing")
		return nil
	}

	rows, err := a.db.Query(`
		SELECT id, claim_id, file_type, file_path, source_file_path, processed_file_path, thumbnail_path
		FROM claim_materials
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type claimMaterial struct {
		id                int64
		claimID           int64
		fileType          string
		filePath          string
		sourceFilePath    string
		processedFilePath string
		thumbnailPath     string
	}

	var items []claimMaterial
	for rows.Next() {
		var item claimMaterial
		if err := rows.Scan(&item.id, &item.claimID, &item.fileType, &item.filePath, &item.sourceFilePath, &item.processedFilePath, &item.thumbnailPath); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range items {
		s := a.summary("claim_materials")
		s.scanned++

		newFilePath := item.filePath
		newSourcePath := item.sourceFilePath
		newProcessedPath := item.processedFilePath
		newThumbnailPath := item.thumbnailPath
		rowChanged := false
		rowCopied := 0

		if isVideoType(item.fileType) && strings.TrimSpace(item.sourceFilePath) != "" {
			jobID, ext := a.claimAssetJobID(item.sourceFilePath, item.claimID, fmt.Sprintf("claim-%d-material-%d", item.claimID, item.id))
			filename := fmt.Sprintf("%s%s", jobID, ext)
			dstKey := storage.BuildUploadObjectKey("video", storage.BizTypeClaimSource, strconv.FormatInt(item.claimID, 10), jobID, ext, filename, 0)
			moved, err := a.applyMigration("claim_materials", item.sourceFilePath, dstKey)
			if err != nil {
				s.failed++
				log.Printf("claim_materials id=%d source: %v", item.id, err)
			} else {
				newSourcePath = moved.url
				rowChanged = rowChanged || moved.changed
				rowCopied += boolToInt(moved.copied)
			}
		}

		if isVideoType(item.fileType) {
			processedRaw := strings.TrimSpace(item.processedFilePath)
			if processedRaw == "" && looksLikeProcessedAsset(a.normalizeObjectKey(item.filePath)) {
				processedRaw = item.filePath
			}
			if processedRaw != "" {
				jobID, ext := a.claimAssetJobID(processedRaw, item.claimID, fmt.Sprintf("claim-%d-material-%d", item.claimID, item.id))
				dstKey := storage.BuildWatermarkedVideoKey(item.claimID, jobID, ext)
				moved, err := a.applyMigration("claim_materials", processedRaw, dstKey)
				if err != nil {
					s.failed++
					log.Printf("claim_materials id=%d processed: %v", item.id, err)
				} else {
					newProcessedPath = moved.url
					if strings.TrimSpace(item.filePath) != "" {
						newFilePath = moved.url
					}
					rowChanged = rowChanged || moved.changed
					rowCopied += boolToInt(moved.copied)
				}
			}
		}

		if strings.TrimSpace(item.thumbnailPath) != "" {
			jobID, ext := a.claimAssetJobID(item.thumbnailPath, item.claimID, fmt.Sprintf("claim-%d-thumb-%d", item.claimID, item.id))
			dstKey := storage.BuildThumbnailKey(item.claimID, jobID, ext)
			moved, err := a.applyMigration("claim_materials", item.thumbnailPath, dstKey)
			if err != nil {
				s.failed++
				log.Printf("claim_materials id=%d thumbnail: %v", item.id, err)
			} else {
				newThumbnailPath = moved.url
				rowChanged = rowChanged || moved.changed
				rowCopied += boolToInt(moved.copied)
			}
		}

		if !rowChanged {
			s.skipped++
			continue
		}
		s.copied += rowCopied
		if a.apply {
			if _, err := a.db.Exec(`
				UPDATE claim_materials
				SET file_path = ?, source_file_path = ?, processed_file_path = ?, thumbnail_path = ?
				WHERE id = ?
			`, newFilePath, newSourcePath, newProcessedPath, newThumbnailPath, item.id); err != nil {
				s.failed++
				log.Printf("claim_materials id=%d update db: %v", item.id, err)
				continue
			}
		}
		s.updated++
	}

	return nil
}

func (a *migrationApp) migrateInspirations() error {
	ok, err := a.hasColumns("inspirations", "id", "created_by", "source_claim_id", "creator_avatar", "cover_url")
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("skip inspirations: required columns are missing")
		return nil
	}

	rows, err := a.db.Query(`
		SELECT id, created_by, source_claim_id, creator_avatar, cover_url
		FROM inspirations
		WHERE TRIM(COALESCE(creator_avatar, '')) <> '' OR TRIM(COALESCE(cover_url, '')) <> ''
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type inspirationRow struct {
		id            int64
		createdBy     int64
		sourceClaimID sql.NullInt64
		creatorAvatar string
		coverURL      string
	}

	var items []inspirationRow
	for rows.Next() {
		var item inspirationRow
		if err := rows.Scan(&item.id, &item.createdBy, &item.sourceClaimID, &item.creatorAvatar, &item.coverURL); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range items {
		s := a.summary("inspirations")
		s.scanned++

		newCreatorAvatar := item.creatorAvatar
		newCoverURL := item.coverURL
		rowChanged := false
		rowCopied := 0

		if strings.TrimSpace(item.creatorAvatar) != "" {
			legacyKey := a.normalizeObjectKey(item.creatorAvatar)
			if isManagedAssetKey(legacyKey) {
				filename := legacyFilename(legacyKey, fmt.Sprintf("avatar-%d%s", item.createdBy, legacyExt(legacyKey, ".jpg")))
				dstKey := storage.BuildUploadObjectKey("image", storage.BizTypeAvatar, strconv.FormatInt(item.createdBy, 10), "", filepath.Ext(filename), filename, item.createdBy)
				moved, err := a.applyMigration("inspirations", item.creatorAvatar, dstKey)
				if err != nil {
					s.failed++
					log.Printf("inspirations id=%d creator_avatar: %v", item.id, err)
				} else {
					newCreatorAvatar = moved.url
					rowChanged = rowChanged || moved.changed
					rowCopied += boolToInt(moved.copied)
				}
			}
		}

		if strings.TrimSpace(item.coverURL) != "" {
			if claimID, ok := chooseClaimID(item.sourceClaimID, a.claimIDFromAsset(item.coverURL)); ok {
				jobID, ext := a.claimAssetJobID(item.coverURL, claimID, fmt.Sprintf("claim-%d-cover-%d", claimID, item.id))
				dstKey := storage.BuildThumbnailKey(claimID, jobID, ext)
				if isVideoFileExt(ext) {
					dstKey = storage.BuildWatermarkedVideoKey(claimID, jobID, ext)
				}
				moved, err := a.applyMigration("inspirations", item.coverURL, dstKey)
				if err != nil {
					s.failed++
					log.Printf("inspirations id=%d cover_url: %v", item.id, err)
				} else {
					newCoverURL = moved.url
					rowChanged = rowChanged || moved.changed
					rowCopied += boolToInt(moved.copied)
				}
			}
		}

		if !rowChanged {
			s.skipped++
			continue
		}
		s.copied += rowCopied
		if a.apply {
			if _, err := a.db.Exec(`UPDATE inspirations SET creator_avatar = ?, cover_url = ?, updated_at = ? WHERE id = ?`, newCreatorAvatar, newCoverURL, time.Now(), item.id); err != nil {
				s.failed++
				log.Printf("inspirations id=%d update db: %v", item.id, err)
				continue
			}
		}
		s.updated++
	}

	return nil
}

func (a *migrationApp) migrateInspirationMaterials() error {
	ok, err := a.hasColumns("inspiration_materials", "id", "inspiration_id", "file_type", "file_path", "thumbnail_path")
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("skip inspiration_materials: required columns are missing")
		return nil
	}

	ok, err = a.hasColumns("inspirations", "id", "source_claim_id")
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("skip inspiration_materials: inspirations.source_claim_id is missing")
		return nil
	}

	rows, err := a.db.Query(`
		SELECT im.id, im.inspiration_id, i.source_claim_id, im.file_type, im.file_path, im.thumbnail_path
		FROM inspiration_materials im
		JOIN inspirations i ON i.id = im.inspiration_id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type inspirationMaterialRow struct {
		id            int64
		inspirationID int64
		sourceClaimID sql.NullInt64
		fileType      string
		filePath      string
		thumbnailPath string
	}

	var items []inspirationMaterialRow
	for rows.Next() {
		var item inspirationMaterialRow
		if err := rows.Scan(&item.id, &item.inspirationID, &item.sourceClaimID, &item.fileType, &item.filePath, &item.thumbnailPath); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range items {
		s := a.summary("inspiration_materials")
		s.scanned++

		newFilePath := item.filePath
		newThumbnailPath := item.thumbnailPath
		rowChanged := false
		rowCopied := 0

		claimID, ok := chooseClaimID(item.sourceClaimID, a.claimIDFromAsset(item.filePath), a.claimIDFromAsset(item.thumbnailPath))
		if !ok {
			s.skipped++
			continue
		}

		if strings.TrimSpace(item.filePath) != "" && (isVideoType(item.fileType) || looksLikeProcessedAsset(a.normalizeObjectKey(item.filePath))) {
			jobID, ext := a.claimAssetJobID(item.filePath, claimID, fmt.Sprintf("claim-%d-inspiration-%d", claimID, item.id))
			dstKey := storage.BuildWatermarkedVideoKey(claimID, jobID, ext)
			if !isVideoType(item.fileType) && !isVideoFileExt(ext) {
				dstKey = storage.BuildThumbnailKey(claimID, jobID, ext)
			}
			moved, err := a.applyMigration("inspiration_materials", item.filePath, dstKey)
			if err != nil {
				s.failed++
				log.Printf("inspiration_materials id=%d file_path: %v", item.id, err)
			} else {
				newFilePath = moved.url
				rowChanged = rowChanged || moved.changed
				rowCopied += boolToInt(moved.copied)
			}
		}

		if strings.TrimSpace(item.thumbnailPath) != "" {
			jobID, ext := a.claimAssetJobID(item.thumbnailPath, claimID, fmt.Sprintf("claim-%d-thumb-%d", claimID, item.id))
			dstKey := storage.BuildThumbnailKey(claimID, jobID, ext)
			moved, err := a.applyMigration("inspiration_materials", item.thumbnailPath, dstKey)
			if err != nil {
				s.failed++
				log.Printf("inspiration_materials id=%d thumbnail_path: %v", item.id, err)
			} else {
				newThumbnailPath = moved.url
				rowChanged = rowChanged || moved.changed
				rowCopied += boolToInt(moved.copied)
			}
		}

		if !rowChanged {
			s.skipped++
			continue
		}
		s.copied += rowCopied
		if a.apply {
			if _, err := a.db.Exec(`UPDATE inspiration_materials SET file_path = ?, thumbnail_path = ? WHERE id = ?`, newFilePath, newThumbnailPath, item.id); err != nil {
				s.failed++
				log.Printf("inspiration_materials id=%d update db: %v", item.id, err)
				continue
			}
		}
		s.updated++
	}

	return nil
}

type migrationResult struct {
	url     string
	changed bool
	copied  bool
}

func (a *migrationApp) applyMigration(stepName, raw, dstKey string) (migrationResult, error) {
	plan, err := a.planMigration(raw, dstKey)
	if err != nil {
		return migrationResult{}, err
	}
	if !plan.needsUpdate && !plan.needsCopy {
		return migrationResult{url: plan.targetURL}, nil
	}

	copied := false
	if a.apply && plan.needsCopy {
		alreadyCopied := a.copiedKeys[plan.srcKey+"->"+plan.dstKey]
		if !alreadyCopied {
			exists, err := a.provider.Exists(a.ctx, plan.dstKey)
			if err != nil {
				return migrationResult{}, fmt.Errorf("check destination exists: %w", err)
			}
			if !exists {
				if err := a.copyToDestination(raw, plan.srcKey, plan.dstKey); err != nil {
					return migrationResult{}, fmt.Errorf("copy object %q -> %q: %w", plan.srcKey, plan.dstKey, err)
				}
				copied = true
			}
			a.copiedKeys[plan.srcKey+"->"+plan.dstKey] = true
		}
	}

	if !a.apply {
		log.Printf("[dry-run] %s: %s -> %s", stepName, raw, plan.targetURL)
	}

	return migrationResult{
		url:     plan.targetURL,
		changed: plan.needsUpdate || plan.needsCopy,
		copied:  copied,
	}, nil
}

func (a *migrationApp) planMigration(raw, dstKey string) (plannedMigration, error) {
	dstKey = strings.TrimLeft(strings.TrimSpace(dstKey), "/")
	if dstKey == "" {
		return plannedMigration{}, fmt.Errorf("empty destination key")
	}

	targetURL, err := a.provider.GetURL(a.ctx, dstKey)
	if err != nil {
		return plannedMigration{}, fmt.Errorf("build target url: %w", err)
	}

	srcKey := a.normalizeObjectKey(raw)
	if srcKey == "" {
		return plannedMigration{
			dstKey:      dstKey,
			targetURL:   targetURL,
			needsUpdate: strings.TrimSpace(raw) != strings.TrimSpace(targetURL),
		}, nil
	}

	return plannedMigration{
		srcKey:      srcKey,
		dstKey:      dstKey,
		targetURL:   targetURL,
		needsCopy:   srcKey != dstKey,
		needsUpdate: strings.TrimSpace(raw) != strings.TrimSpace(targetURL),
	}, nil
}

func (a *migrationApp) copyToDestination(raw, srcKey, dstKey string) error {
	if localPath := a.legacyLocalFilePath(raw); localPath != "" {
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			return a.uploadLocalFile(localPath, dstKey, info.Size())
		}
	}

	if downloadURL := a.legacyDownloadURL(raw); downloadURL != "" {
		if err := a.uploadRemoteFile(downloadURL, dstKey); err == nil {
			return nil
		}
	}

	return storage.CopyObject(a.ctx, a.provider, srcKey, dstKey)
}

func (a *migrationApp) normalizeObjectKey(raw string) string {
	key := strings.TrimLeft(strings.TrimSpace(storage.ExtractObjectKey(raw, a.bucket)), "/")
	key = strings.TrimPrefix(key, "static/uploads/")
	key = strings.TrimPrefix(key, "uploads/")
	return key
}

func (a *migrationApp) legacyLocalFilePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	assetPath := raw
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		assetPath = parsed.Path
	}

	switch {
	case strings.HasPrefix(assetPath, "/static/uploads/"):
		assetPath = strings.TrimPrefix(assetPath, "/static/uploads/")
	case strings.HasPrefix(assetPath, "static/uploads/"):
		assetPath = strings.TrimPrefix(assetPath, "static/uploads/")
	case strings.HasPrefix(assetPath, "/uploads/"):
		assetPath = strings.TrimPrefix(assetPath, "/uploads/")
	case strings.HasPrefix(assetPath, "uploads/"):
		assetPath = strings.TrimPrefix(assetPath, "uploads/")
	default:
		return ""
	}

	return filepath.Join(a.workDir, "web", "static", "uploads", filepath.FromSlash(assetPath))
}

func (a *migrationApp) legacyDownloadURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "/") {
		return strings.TrimRight(a.cfg.Static.Host, "/") + raw
	}
	return ""
}

func (a *migrationApp) uploadLocalFile(localPath, dstKey string, size int64) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(localPath)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = a.provider.Upload(a.ctx, dstKey, file, size, contentType)
	return err
}

func (a *migrationApp) uploadRemoteFile(sourceURL, dstKey string) error {
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download source returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(sourceURL)))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = a.provider.Upload(a.ctx, dstKey, bytes.NewReader(data), int64(len(data)), contentType)
	return err
}

func (a *migrationApp) claimAssetJobID(raw string, claimID int64, fallback string) (string, string) {
	key := a.normalizeObjectKey(raw)
	ext := legacyExt(key, ".mp4")
	segments := strings.Split(key, "/")
	for i := 0; i+2 < len(segments); i++ {
		switch {
		case segments[i] == "claim-source" || segments[i] == "claim-processed":
			if parsed, err := strconv.ParseInt(segments[i+1], 10, 64); err == nil && parsed == claimID {
				base := path.Base(segments[i+2])
				jobID := strings.TrimSuffix(base, filepath.Ext(base))
				if jobID != "" {
					return jobID, ext
				}
			}
		case segments[i] == "private" && i+3 < len(segments) && segments[i+1] == "source":
			if parsed, err := strconv.ParseInt(segments[i+2], 10, 64); err == nil && parsed == claimID {
				base := path.Base(segments[i+3])
				jobID := strings.TrimSuffix(base, filepath.Ext(base))
				if jobID != "" {
					return jobID, ext
				}
			}
		case segments[i] == "public" && i+3 < len(segments) && (segments[i+1] == "watermarked" || segments[i+1] == "thumbnails"):
			if parsed, err := strconv.ParseInt(segments[i+2], 10, 64); err == nil && parsed == claimID {
				base := path.Base(segments[i+3])
				jobID := strings.TrimSuffix(base, filepath.Ext(base))
				if jobID != "" {
					return jobID, ext
				}
			}
		}
	}
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		fallback = fmt.Sprintf("claim-%d-%d", claimID, time.Now().UnixNano())
	}
	return fallback, ext
}

func (a *migrationApp) claimIDFromAsset(raw string) int64 {
	key := a.normalizeObjectKey(raw)
	segments := strings.Split(key, "/")
	for i := 0; i+1 < len(segments); i++ {
		switch {
		case segments[i] == "claim-source" || segments[i] == "claim-processed":
			if id, err := strconv.ParseInt(segments[i+1], 10, 64); err == nil {
				return id
			}
		case segments[i] == "private" && i+2 < len(segments) && segments[i+1] == "source":
			if id, err := strconv.ParseInt(segments[i+2], 10, 64); err == nil {
				return id
			}
		case segments[i] == "public" && i+2 < len(segments) && (segments[i+1] == "watermarked" || segments[i+1] == "thumbnails"):
			if id, err := strconv.ParseInt(segments[i+2], 10, 64); err == nil {
				return id
			}
		}
	}
	return 0
}

func (a *migrationApp) summary(name string) *migrationSummary {
	if a.summaries[name] == nil {
		a.summaries[name] = &migrationSummary{}
	}
	return a.summaries[name]
}

func (a *migrationApp) hasColumns(table string, columns ...string) (bool, error) {
	rows, err := a.db.Query("SELECT * FROM " + table + " LIMIT 0")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return false, err
	}

	existing := make(map[string]struct{}, len(names))
	for _, name := range names {
		existing[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	for _, column := range columns {
		if _, ok := existing[strings.ToLower(strings.TrimSpace(column))]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func initStorageProvider(cfg *config.Config, workDir string) (storage.StorageProvider, error) {
	factory := storage.NewFactory(cfg.Static.Host, cfg.Static.CDN, workDir)

	switch strings.ToLower(strings.TrimSpace(cfg.Storage.Provider)) {
	case "rustfs":
		return factory.NewProvider(storage.Config{
			Type: storage.StorageTypeRustFS,
			RustFS: storage.S3CompatibleConfig{
				Endpoint:          cfg.Storage.RustFS.Endpoint,
				Bucket:            cfg.Storage.RustFS.Bucket,
				AccessKey:         cfg.Storage.RustFS.AccessKey,
				SecretKey:         cfg.Storage.RustFS.SecretKey,
				Region:            cfg.Storage.RustFS.Region,
				UsePathStyle:      true,
				HostnameImmutable: false,
			},
		})
	case "s3":
		return factory.NewProvider(storage.Config{
			Type: storage.StorageTypeS3,
			S3: storage.S3Config{
				Endpoint:        cfg.Storage.S3.Endpoint,
				Bucket:          cfg.Storage.S3.Bucket,
				Region:          cfg.Storage.S3.Region,
				AccessKeyID:     cfg.Storage.S3.AccessKeyID,
				SecretAccessKey: cfg.Storage.S3.SecretAccessKey,
			},
		})
	case "oss":
		return factory.NewProvider(storage.Config{
			Type: storage.StorageTypeOSS,
			OSS: storage.OSSConfig{
				Endpoint:    cfg.Storage.OSS.Endpoint,
				Bucket:      cfg.Storage.OSS.Bucket,
				AccessKeyID: cfg.Storage.OSS.AccessKey,
				SecretKey:   cfg.Storage.OSS.SecretKey,
				CDNHost:     cfg.Storage.OSS.CDNHost,
			},
		})
	case "cos":
		return factory.NewProvider(storage.Config{
			Type: storage.StorageTypeCOS,
			COS: storage.COSConfig{
				AppID:     cfg.Storage.COS.AppID,
				Bucket:    cfg.Storage.COS.Bucket,
				Region:    cfg.Storage.COS.Region,
				SecretKey: cfg.Storage.COS.SecretKey,
				SecretID:  cfg.Storage.COS.SecretID,
				CDNHost:   cfg.Storage.COS.CDNHost,
			},
		})
	default:
		return factory.NewProvider(storage.Config{
			Type: storage.StorageTypeLocal,
			Local: storage.LocalConfig{
				BaseDir: "",
				BaseURL: "",
			},
		})
	}
}

func configuredStorageBucket(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Storage.Provider)) {
	case "cos":
		if cfg.Storage.COS.AppID != "" && !strings.Contains(cfg.Storage.COS.Bucket, "-") {
			return cfg.Storage.COS.Bucket + "-" + cfg.Storage.COS.AppID
		}
		return cfg.Storage.COS.Bucket
	case "s3":
		return cfg.Storage.S3.Bucket
	case "oss":
		return cfg.Storage.OSS.Bucket
	case "rustfs":
		return cfg.Storage.RustFS.Bucket
	default:
		return ""
	}
}

func isManagedAssetKey(key string) bool {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	switch {
	case strings.HasPrefix(key, "public/"),
		strings.HasPrefix(key, "private/"),
		strings.HasPrefix(key, "image/"),
		strings.HasPrefix(key, "video/"),
		strings.HasPrefix(key, "claim-source/"),
		strings.HasPrefix(key, "claim-processed/"):
		return true
	default:
		return false
	}
}

func looksLikeProcessedAsset(key string) bool {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	return strings.HasPrefix(key, "claim-processed/") || strings.HasPrefix(key, "public/watermarked/")
}

func isVideoType(fileType string) bool {
	fileType = strings.ToLower(strings.TrimSpace(fileType))
	return fileType == "video" || strings.HasPrefix(fileType, "video/")
}

func isVideoFileExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".mp4", ".mov", ".avi", ".wmv", ".flv", ".mkv", ".webm":
		return true
	default:
		return false
	}
}

func legacyExt(key, fallback string) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(strings.TrimSpace(key))))
	if ext == "" {
		ext = fallback
	}
	if ext == "" {
		ext = ".jpg"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

func legacyFilename(key, fallback string) string {
	base := path.Base(strings.TrimSpace(key))
	if base == "." || base == "/" || base == "" {
		return fallback
	}
	return base
}

func chooseClaimID(values ...interface{}) (int64, bool) {
	for _, value := range values {
		switch v := value.(type) {
		case sql.NullInt64:
			if v.Valid && v.Int64 > 0 {
				return v.Int64, true
			}
		case int64:
			if v > 0 {
				return v, true
			}
		}
	}
	return 0, false
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
