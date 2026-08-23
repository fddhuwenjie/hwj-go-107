package service

import (
	"context"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

// ArtifactService 藏品服务。
type ArtifactService struct {
	d *Deps
}

// NewArtifactService 构造藏品服务。
func NewArtifactService(d *Deps) *ArtifactService { return &ArtifactService{d: d} }

// RegisterInput 登记入参。
type RegisterInput struct {
	Code, Name, Category, Era, Description string
	LevelID                                int64
}

// Register 登记藏品，初始状态 registered。
func (s *ArtifactService) Register(ctx context.Context, in RegisterInput) (*domain.Artifact, error) {
	if strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Name) == "" {
		return nil, domain.Invalidf("藏品编号与名称不能为空")
	}
	if _, err := s.d.Repo.Levels.GetByID(ctx, in.LevelID); err != nil {
		return nil, err
	}
	now := s.d.now()
	a := &domain.Artifact{
		Code: in.Code, Name: in.Name, Category: in.Category, Era: in.Era,
		Description: in.Description, Status: domain.ArtifactRegistered,
		LevelID: in.LevelID, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		if err := r.Artifacts.Create(ctx, a); err != nil {
			return err
		}
		if err := s.d.Audit.SnapshotArtifact(ctx, r, a, "登记藏品"); err != nil {
			return err
		}
		return s.d.Audit.Record(ctx, r, "system", "artifact.register", "artifact", a.ID, a.Code)
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

// UpdateInput 修改基础信息入参。
type UpdateInput struct {
	Name, Category, Era, Description, Note string
	Version                                int64
}

// Update 修改藏品基础信息（乐观锁）；借展占用与已注销状态拒绝修改。
func (s *ArtifactService) Update(ctx context.Context, id int64, in UpdateInput) (*domain.Artifact, error) {
	var out *domain.Artifact
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		a, err := r.Artifacts.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if !a.Editable() {
			return domain.Statef("藏品当前状态 %s 不允许直接修改", a.Status)
		}
		if in.Version != a.Version {
			return domain.Conflictf("藏品版本冲突：期望 %d 实际 %d", in.Version, a.Version)
		}
		a.Name, a.Category, a.Era, a.Description = in.Name, in.Category, in.Era, in.Description
		a.Note = in.Note
		a.UpdatedAt = s.d.now()
		if err := r.Artifacts.Update(ctx, a); err != nil {
			return err
		}
		if err := s.d.Audit.Record(ctx, r, "system", "artifact.update", "artifact", a.ID, ""); err != nil {
			return err
		}
		out = a
		return nil
	})
	return out, err
}

// AssignLocation 分配/变更库位，registered -> stored。
func (s *ArtifactService) AssignLocation(ctx context.Context, id, unitID, version int64) (*domain.Artifact, error) {
	var out *domain.Artifact
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		a, err := r.Artifacts.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if a.Status != domain.ArtifactRegistered && a.Status != domain.ArtifactStored {
			return domain.Statef("藏品状态 %s 不允许分配库位", a.Status)
		}
		u, err := r.Units.GetByID(ctx, unitID)
		if err != nil {
			return err
		}
		if u.Kind != domain.UnitWarehouse || u.Status != domain.UnitActive {
			return domain.Rulef("目标单元 %d 不是启用状态的库房", unitID)
		}
		a.StorageUnitID = &unitID
		a.Status = domain.ArtifactStored
		a.UpdatedAt = s.d.now()
		a.Version = version
		if err := r.Artifacts.Update(ctx, a); err != nil {
			return err
		}
		if err := s.d.Audit.SnapshotArtifact(ctx, r, a, "分配库位"); err != nil {
			return err
		}
		out = a
		return s.d.Audit.Record(ctx, r, "system", "artifact.assign_location", "artifact", a.ID, u.Code)
	})
	return out, err
}

// Retire 受约束注销：仅 registered/stored 允许，保留审计与历史。
func (s *ArtifactService) Retire(ctx context.Context, id, version int64, reason string) (*domain.Artifact, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, domain.Invalidf("注销理由不能为空")
	}
	var out *domain.Artifact
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		a, err := r.Artifacts.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if !a.CanRetire() {
			return domain.Statef("藏品状态 %s 不允许注销（进行中借展或已注销）", a.Status)
		}
		a.Version = version
		a.Retired = true
		a.RetiredReason = reason
		a.Status = domain.ArtifactRetired
		a.UpdatedAt = s.d.now()
		if err := r.Artifacts.Update(ctx, a); err != nil {
			return err
		}
		if err := s.d.Audit.SnapshotArtifact(ctx, r, a, "注销："+reason); err != nil {
			return err
		}
		out = a
		return s.d.Audit.Record(ctx, r, "system", "artifact.retire", "artifact", a.ID, reason)
	})
	return out, err
}

// Get 详情。
func (s *ArtifactService) Get(ctx context.Context, id int64) (*domain.Artifact, error) {
	return s.d.Repo.Artifacts.GetByID(ctx, id)
}

// List 稳定分页。
func (s *ArtifactService) List(ctx context.Context, f repository.ArtifactFilter, p domain.Page) (domain.Paged[domain.Artifact], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Artifacts.List(ctx, f, p)
	if err != nil {
		return domain.Paged[domain.Artifact]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(a domain.Artifact) int64 { return a.ID }), nil
}

// AddAttachment 登记附件。
func (s *ArtifactService) AddAttachment(ctx context.Context, artifactID int64, name, spec string) (*domain.Attachment, error) {
	if strings.TrimSpace(name) == "" {
		return nil, domain.Invalidf("附件名称不能为空")
	}
	a, err := s.d.Repo.Artifacts.GetByID(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	if !a.Editable() {
		return nil, domain.Statef("藏品状态 %s 不允许登记附件", a.Status)
	}
	at := &domain.Attachment{ArtifactID: artifactID, Name: name, Spec: spec, CreatedAt: s.d.now()}
	if err := s.d.Repo.Attachments.Create(ctx, at); err != nil {
		return nil, err
	}
	return at, nil
}

// ListAttachments 附件列表。
func (s *ArtifactService) ListAttachments(ctx context.Context, artifactID int64) ([]domain.Attachment, error) {
	items, err := s.d.Repo.Attachments.ListByArtifact(ctx, artifactID)
	if err != nil { return nil, err }
	for i := range items {
		items[i].ArtifactID = artifactID
	}
	return items, nil
}

// Snapshots 状态历史快照分页。
func (s *ArtifactService) Snapshots(ctx context.Context, artifactID int64, p domain.Page) (domain.Paged[domain.ArtifactSnapshot], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Snapshots.ListByArtifact(ctx, artifactID, p)
	if err != nil {
		return domain.Paged[domain.ArtifactSnapshot]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(s domain.ArtifactSnapshot) int64 { return s.ID }), nil
}
