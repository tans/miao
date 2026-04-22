package service

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

type ClaimInspirationService struct {
	inspirationRepo *repository.InspirationRepository
}

func NewClaimInspirationService(db *sql.DB) *ClaimInspirationService {
	return &ClaimInspirationService{
		inspirationRepo: repository.NewInspirationRepository(db),
	}
}

func (s *ClaimInspirationService) SaveDraftFromClaim(claim *model.Claim, task *model.Task, creator *model.User, materials []*model.ClaimMaterial) (*model.Inspiration, error) {
	return s.syncFromClaim(claim, task, creator, materials, false)
}

func (s *ClaimInspirationService) PublishFromClaim(claim *model.Claim, task *model.Task, creator *model.User, materials []*model.ClaimMaterial) (*model.Inspiration, error) {
	return s.syncFromClaim(claim, task, creator, materials, true)
}

func (s *ClaimInspirationService) DeleteByClaimID(sourceClaimID int64) error {
	return s.inspirationRepo.DeleteBySourceClaimID(sourceClaimID)
}

func (s *ClaimInspirationService) syncFromClaim(claim *model.Claim, task *model.Task, creator *model.User, materials []*model.ClaimMaterial, published bool) (*model.Inspiration, error) {
	if s == nil || s.inspirationRepo == nil {
		return nil, fmt.Errorf("inspiration service not initialized")
	}
	if claim == nil || task == nil || creator == nil {
		return nil, fmt.Errorf("missing claim context")
	}
	if len(materials) == 0 {
		return nil, fmt.Errorf("claim %d has no materials", claim.ID)
	}

	existing, err := s.inspirationRepo.GetBySourceClaimID(claim.ID)
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}

	now := time.Now()
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = "灵感作品"
	}

	item := &model.Inspiration{
		Title:         title,
		Content:       strings.TrimSpace(claim.Content),
		Tags:          normalizeClaimTags(task.Industries),
		CreatorName:   normalizeCreatorName(creator),
		CreatorAvatar: strings.TrimSpace(creator.Avatar),
		CoverURL:      coverURLFromClaimMaterials(materials),
		CoverType:     coverTypeFromClaimMaterials(materials),
		SortOrder:     0,
		Status:        model.InspirationStatusDraft,
		PublishedAt:   nil,
		CreatedBy:     creator.ID,
	}
	sourceClaimID := claim.ID
	item.SourceClaimID = &sourceClaimID

	if existing != nil {
		item.ID = existing.ID
		item.CreatedBy = existing.CreatedBy
		item.Views = existing.Views
		item.Likes = existing.Likes
		item.SortOrder = existing.SortOrder
		item.SourceClaimID = existing.SourceClaimID
	}

	if published {
		item.Status = model.InspirationStatusPublished
		item.PublishedAt = &now
	}

	if existing != nil {
		if err := s.inspirationRepo.Update(item); err != nil {
			return nil, err
		}
	} else {
		if err := s.inspirationRepo.Create(item); err != nil {
			return nil, err
		}
	}

	inputs := make([]model.InspirationMaterialInput, 0, len(materials))
	order := 1
	for _, material := range materials {
		if material == nil {
			continue
		}
		inputs = append(inputs, model.InspirationMaterialInput{
			FileName:      material.FileName,
			FilePath:      material.FilePath,
			FileSize:      material.FileSize,
			FileType:      material.FileType,
			ThumbnailPath: material.ThumbnailPath,
			SortOrder:     order,
		})
		order++
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("claim %d has no valid materials", claim.ID)
	}

	if err := s.inspirationRepo.ReplaceMaterials(item.ID, inputs); err != nil {
		if existing == nil {
			_ = s.inspirationRepo.Delete(item.ID)
		}
		return nil, err
	}

	latest, err := s.inspirationRepo.GetByID(item.ID)
	if err == nil && latest != nil {
		mats, matErr := s.inspirationRepo.GetMaterials(item.ID)
		if matErr == nil {
			latest.Materials = mats
		}
		return latest, nil
	}
	return item, nil
}

func normalizeClaimTags(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "，", ","))
	if raw == "" {
		return ""
	}

	parts := strings.Split(raw, ",")
	seen := map[string]struct{}{}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return strings.Join(result, ",")
}

func normalizeCreatorName(creator *model.User) string {
	if creator == nil {
		return "创意喵"
	}
	name := strings.TrimSpace(creator.Nickname)
	if name == "" {
		name = strings.TrimSpace(creator.Username)
	}
	if name == "" {
		name = "创意喵"
	}
	return name
}

func coverURLFromClaimMaterials(materials []*model.ClaimMaterial) string {
	if len(materials) == 0 || materials[0] == nil {
		return ""
	}
	if thumb := strings.TrimSpace(materials[0].ThumbnailPath); thumb != "" {
		return thumb
	}
	return strings.TrimSpace(materials[0].FilePath)
}

func coverTypeFromClaimMaterials(materials []*model.ClaimMaterial) string {
	if len(materials) == 0 || materials[0] == nil {
		return "image"
	}
	coverType := strings.TrimSpace(materials[0].FileType)
	if coverType == "" {
		coverType = "image"
	}
	return coverType
}
