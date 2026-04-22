package repository

import (
	"database/sql"
	"strings"
	"time"

	"github.com/tans/miao/internal/model"
)

type InspirationRepository struct {
	db *sql.DB
}

func NewInspirationRepository(db *sql.DB) *InspirationRepository {
	return &InspirationRepository{db: db}
}

func (r *InspirationRepository) Create(inspiration *model.Inspiration) error {
	query := `
		INSERT INTO inspirations (
			title, content, tags, creator_name, creator_avatar, cover_url, cover_width, cover_height, cover_type,
			status, views, likes, sort_order, created_by, source_claim_id, published_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		inspiration.Title,
		inspiration.Content,
		inspiration.Tags,
		inspiration.CreatorName,
		inspiration.CreatorAvatar,
		inspiration.CoverURL,
		inspiration.CoverWidth,
		inspiration.CoverHeight,
		inspiration.CoverType,
		inspiration.Status,
		inspiration.Views,
		inspiration.Likes,
		inspiration.SortOrder,
		inspiration.CreatedBy,
		inspiration.SourceClaimID,
		inspiration.PublishedAt,
		now,
		now,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	inspiration.ID = id
	inspiration.CreatedAt = now
	inspiration.UpdatedAt = now
	return nil
}

func (r *InspirationRepository) Update(inspiration *model.Inspiration) error {
	query := `
		UPDATE inspirations
		SET title = ?, content = ?, tags = ?, creator_name = ?, creator_avatar = ?, cover_url = ?, cover_width = ?, cover_height = ?, cover_type = ?,
			status = ?, sort_order = ?, published_at = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		inspiration.Title,
		inspiration.Content,
		inspiration.Tags,
		inspiration.CreatorName,
		inspiration.CreatorAvatar,
		inspiration.CoverURL,
		inspiration.CoverWidth,
		inspiration.CoverHeight,
		inspiration.CoverType,
		inspiration.Status,
		inspiration.SortOrder,
		inspiration.PublishedAt,
		time.Now(),
		inspiration.ID,
	)
	return err
}

func (r *InspirationRepository) Delete(id int64) error {
	if _, err := r.db.Exec(`DELETE FROM inspiration_materials WHERE inspiration_id = ?`, id); err != nil {
		return err
	}
	_, err := r.db.Exec(`DELETE FROM inspirations WHERE id = ?`, id)
	return err
}

func (r *InspirationRepository) DeleteBySourceClaimID(sourceClaimID int64) error {
	if _, err := r.db.Exec(`DELETE FROM inspiration_materials WHERE inspiration_id IN (SELECT id FROM inspirations WHERE source_claim_id = ?)`, sourceClaimID); err != nil {
		return err
	}
	_, err := r.db.Exec(`DELETE FROM inspirations WHERE source_claim_id = ?`, sourceClaimID)
	return err
}

func (r *InspirationRepository) GetByID(id int64) (*model.Inspiration, error) {
	query := `
		SELECT id, title, content, tags, creator_name, creator_avatar, cover_url, cover_width, cover_height, cover_type,
			status, views, likes, sort_order, created_by, source_claim_id, published_at, created_at, updated_at
		FROM inspirations WHERE id = ?
	`
	return r.scanOne(query, id)
}

func (r *InspirationRepository) GetBySourceClaimID(sourceClaimID int64) (*model.Inspiration, error) {
	query := `
		SELECT id, title, content, tags, creator_name, creator_avatar, cover_url, cover_width, cover_height, cover_type,
			status, views, likes, sort_order, created_by, source_claim_id, published_at, created_at, updated_at
		FROM inspirations WHERE source_claim_id = ? LIMIT 1
	`
	return r.scanOne(query, sourceClaimID)
}

func (r *InspirationRepository) ListPublic(keyword, tag, sort string, limit, offset int) ([]*model.Inspiration, int, error) {
	whereClause := `WHERE status = ?`
	args := []interface{}{model.InspirationStatusPublished}

	if keyword != "" {
		like := "%" + escapeLikeKeyword(keyword) + "%"
		whereClause += ` AND (title LIKE ? OR content LIKE ? OR creator_name LIKE ?)`
		args = append(args, like, like, like)
	}
	if tag != "" {
		whereClause += ` AND instr(',' || replace(tags, '，', ',') || ',', ',' || ? || ',') > 0`
		args = append(args, strings.TrimSpace(tag))
	}

	countQuery := `SELECT COUNT(*) FROM inspirations ` + whereClause
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := publicInspirationOrder("", sort)
	query := `
		SELECT id, title, content, tags, creator_name, creator_avatar, cover_url, cover_width, cover_height, cover_type,
			status, views, likes, sort_order, created_by, source_claim_id, published_at, created_at, updated_at
		FROM inspirations ` + whereClause + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	items, err := r.scanMany(query, args...)
	return items, total, err
}

func (r *InspirationRepository) ListByBusinessID(businessID int64, keyword, tag, sort string, limit, offset int) ([]*model.Inspiration, int, error) {
	whereClause := `
		WHERE i.status = ?
		  AND i.source_claim_id IS NOT NULL
		  AND t.business_id = ?
	`
	args := []interface{}{model.InspirationStatusPublished, businessID}

	if keyword != "" {
		like := "%" + escapeLikeKeyword(keyword) + "%"
		whereClause += ` AND (i.title LIKE ? OR i.content LIKE ? OR i.creator_name LIKE ?)`
		args = append(args, like, like, like)
	}
	if tag != "" {
		whereClause += ` AND instr(',' || replace(i.tags, '，', ',') || ',', ',' || ? || ',') > 0`
		args = append(args, strings.TrimSpace(tag))
	}

	countQuery := `
		SELECT COUNT(*)
		FROM inspirations i
		JOIN claims c ON c.id = i.source_claim_id
		JOIN tasks t ON t.id = c.task_id
	` + whereClause
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := publicInspirationOrder("i.", sort)
	query := `
		SELECT i.id, i.title, i.content, i.tags, i.creator_name, i.creator_avatar, i.cover_url, i.cover_width, i.cover_height, i.cover_type,
			i.status, i.views, i.likes, i.sort_order, i.created_by, i.source_claim_id, i.published_at, i.created_at, i.updated_at
		FROM inspirations i
		JOIN claims c ON c.id = i.source_claim_id
		JOIN tasks t ON t.id = c.task_id
	` + whereClause + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	items, err := r.scanMany(query, args...)
	return items, total, err
}

func (r *InspirationRepository) ListAdmin(keyword, tag string, status *int, limit, offset int) ([]*model.Inspiration, int, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	if keyword != "" {
		like := "%" + escapeLikeKeyword(keyword) + "%"
		whereClause += " AND (title LIKE ? OR content LIKE ? OR creator_name LIKE ?)"
		args = append(args, like, like, like)
	}
	if tag != "" {
		whereClause += ` AND instr(',' || replace(tags, '，', ',') || ',', ',' || ? || ',') > 0`
		args = append(args, strings.TrimSpace(tag))
	}
	if status != nil {
		whereClause += " AND status = ?"
		args = append(args, *status)
	}

	countQuery := `SELECT COUNT(*) FROM inspirations ` + whereClause
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, title, content, tags, creator_name, creator_avatar, cover_url, cover_width, cover_height, cover_type,
			status, views, likes, sort_order, created_by, source_claim_id, published_at, created_at, updated_at
		FROM inspirations ` + whereClause + ` ORDER BY sort_order DESC, created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	items, err := r.scanMany(query, args...)
	return items, total, err
}

func (r *InspirationRepository) IncrementViews(id int64) error {
	_, err := r.db.Exec(`UPDATE inspirations SET views = views + 1, updated_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (r *InspirationRepository) HasLiked(inspirationID, userID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM inspiration_likes WHERE inspiration_id = ? AND user_id = ?`, inspirationID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *InspirationRepository) AddLike(inspirationID, userID int64) (bool, error) {
	liked, err := r.HasLiked(inspirationID, userID)
	if err != nil {
		return false, err
	}
	if liked {
		return false, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO inspiration_likes (inspiration_id, user_id, created_at) VALUES (?, ?, ?)`, inspirationID, userID, time.Now()); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`UPDATE inspirations SET likes = likes + 1, updated_at = ? WHERE id = ?`, time.Now(), inspirationID); err != nil {
		return false, err
	}

	return true, tx.Commit()
}

func (r *InspirationRepository) RemoveLike(inspirationID, userID int64) (bool, error) {
	liked, err := r.HasLiked(inspirationID, userID)
	if err != nil {
		return false, err
	}
	if !liked {
		return false, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM inspiration_likes WHERE inspiration_id = ? AND user_id = ?`, inspirationID, userID); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`UPDATE inspirations SET likes = CASE WHEN likes > 0 THEN likes - 1 ELSE 0 END, updated_at = ? WHERE id = ?`, time.Now(), inspirationID); err != nil {
		return false, err
	}

	return true, tx.Commit()
}

func (r *InspirationRepository) ReplaceMaterials(inspirationID int64, materials []model.InspirationMaterialInput) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM inspiration_materials WHERE inspiration_id = ?`, inspirationID); err != nil {
		return err
	}

	query := `
		INSERT INTO inspiration_materials (
			inspiration_id, file_name, file_path, file_size, file_type, thumbnail_path, sort_order, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	for idx, material := range materials {
		sortOrder := material.SortOrder
		if sortOrder == 0 {
			sortOrder = idx + 1
		}
		if _, err := tx.Exec(query,
			inspirationID,
			material.FileName,
			material.FilePath,
			material.FileSize,
			material.FileType,
			material.ThumbnailPath,
			sortOrder,
			now,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *InspirationRepository) GetMaterials(inspirationID int64) ([]*model.InspirationMaterial, error) {
	query := `
		SELECT id, inspiration_id, file_name, file_path, file_size, file_type, thumbnail_path, sort_order, created_at
		FROM inspiration_materials WHERE inspiration_id = ? ORDER BY sort_order ASC, id ASC
	`
	rows, err := r.db.Query(query, inspirationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var materials []*model.InspirationMaterial
	for rows.Next() {
		material := &model.InspirationMaterial{}
		if err := rows.Scan(
			&material.ID,
			&material.InspirationID,
			&material.FileName,
			&material.FilePath,
			&material.FileSize,
			&material.FileType,
			&material.ThumbnailPath,
			&material.SortOrder,
			&material.CreatedAt,
		); err != nil {
			return nil, err
		}
		materials = append(materials, material)
	}
	return materials, rows.Err()
}

func (r *InspirationRepository) scanOne(query string, args ...interface{}) (*model.Inspiration, error) {
	item := &model.Inspiration{}
	var content, tags, creatorName, creatorAvatar, coverURL, coverType sql.NullString
	var sourceClaimID sql.NullInt64
	var publishedAt sql.NullTime
	err := r.db.QueryRow(query, args...).Scan(
		&item.ID,
		&item.Title,
		&content,
		&tags,
		&creatorName,
		&creatorAvatar,
		&coverURL,
		&item.CoverWidth,
		&item.CoverHeight,
		&coverType,
		&item.Status,
		&item.Views,
		&item.Likes,
		&item.SortOrder,
		&item.CreatedBy,
		&sourceClaimID,
		&publishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	item.Content = content.String
	item.Tags = tags.String
	item.CreatorName = creatorName.String
	item.CreatorAvatar = creatorAvatar.String
	item.CoverURL = coverURL.String
	item.CoverType = coverType.String
	if sourceClaimID.Valid {
		v := sourceClaimID.Int64
		item.SourceClaimID = &v
	}
	if publishedAt.Valid {
		item.PublishedAt = &publishedAt.Time
	}
	return item, nil
}

func (r *InspirationRepository) scanMany(query string, args ...interface{}) ([]*model.Inspiration, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*model.Inspiration
	for rows.Next() {
		item := &model.Inspiration{}
		var content, tags, creatorName, creatorAvatar, coverURL, coverType sql.NullString
		var sourceClaimID sql.NullInt64
		var publishedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&content,
			&tags,
			&creatorName,
			&creatorAvatar,
			&coverURL,
			&item.CoverWidth,
			&item.CoverHeight,
			&coverType,
			&item.Status,
			&item.Views,
			&item.Likes,
			&item.SortOrder,
			&item.CreatedBy,
			&sourceClaimID,
			&publishedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Content = content.String
		item.Tags = tags.String
		item.CreatorName = creatorName.String
		item.CreatorAvatar = creatorAvatar.String
		item.CoverURL = coverURL.String
		item.CoverType = coverType.String
		if sourceClaimID.Valid {
			v := sourceClaimID.Int64
			item.SourceClaimID = &v
		}
		if publishedAt.Valid {
			item.PublishedAt = &publishedAt.Time
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func publicInspirationOrder(prefix, sort string) string {
	withPrefix := func(column string) string {
		if prefix == "" {
			return column
		}
		return prefix + column
	}

	switch strings.ToLower(sort) {
	case "likes":
		return withPrefix("likes") + " DESC, " +
			withPrefix("sort_order") + " DESC, " +
			withPrefix("published_at") + " DESC, " +
			withPrefix("created_at") + " DESC"
	case "views":
		return withPrefix("views") + " DESC, " +
			withPrefix("sort_order") + " DESC, " +
			withPrefix("published_at") + " DESC, " +
			withPrefix("created_at") + " DESC"
	default:
		return withPrefix("sort_order") + " DESC, " +
			withPrefix("published_at") + " DESC, " +
			withPrefix("created_at") + " DESC"
	}
}
