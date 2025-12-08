package service

import (
	"context"

	"short-link/internal/dto"
	"short-link/internal/model"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/response"
	"short-link/internal/repository"

	"gorm.io/gorm"
)

type TagService interface {
	Add(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.AddTagRequest) error
	Remove(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.RemoveTagRequest) error
	List(ctx context.Context, user *jwt.UserInfo) (*dto.TagListResponse, error)
}

type tagService struct {
	db            *gorm.DB // 注入 gorm.DB 以便开启事务
	tagRepo       repository.TagRepository
	shortlinkRepo repository.ShortlinkRepository // 依赖 shortlinkRepo 进行权限校验
}

func NewTagService(db *gorm.DB, tagRepo repository.TagRepository, shortlinkRepo repository.ShortlinkRepository) TagService {
	return &tagService{
		db:            db,
		tagRepo:       tagRepo,
		shortlinkRepo: shortlinkRepo,
	}
}

// checkOwnership 是一个内部辅助函数，用于校验用户是否为短链接的所有者
func (s *tagService) checkOwnership(ctx context.Context, user *jwt.UserInfo, shortCode string) error {
	shortlink, err := s.shortlinkRepo.GetUniqueShortCode(ctx, shortCode, false)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
		}
		return bizErrors.New(response.InternalError, "数据库查询失败")
	}
	if shortlink.UserId != int64(user.ID) {
		return bizErrors.New(response.Forbidden, "无权操作此链接")
	}
	return nil
}

func (s *tagService) Add(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.AddTagRequest) error {
	// 1. 权限校验
	if err := s.checkOwnership(ctx, user, shortCode); err != nil {
		return err
	}

	// 2. 智能添加逻辑，解决软删除和唯一键冲突
	existingTag, err := s.tagRepo.FindByUK(ctx, user.ID, shortCode, req.TagName)

	// 场景一：完全不存在，则创建
	if err != nil && err == gorm.ErrRecordNotFound {
		newTag := &model.Tag{
			ShortCode: shortCode,
			UserId:    int64(user.ID),
			TagName:   req.TagName,
		}
		return s.tagRepo.Create(ctx, newTag)
	}
	// 处理查询时可能发生的其他数据库错误
	if err != nil {
		return bizErrors.New(response.InternalError, "数据库查询失败")
	}

	// 场景二：存在但已被软删除，则恢复
	if existingTag.DeletedAt.Valid {
		return s.tagRepo.Undelete(ctx, existingTag.ID)
	}

	// 场景三：存在且是激活状态，直接返回成功（幂等性）
	return nil
}

func (s *tagService) Remove(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.RemoveTagRequest) error {
	// 1. 权限校验
	if err := s.checkOwnership(ctx, user, shortCode); err != nil {
		return err
	}

	// 2. 执行删除
	err := s.tagRepo.Delete(ctx, user.ID, shortCode, req.TagName)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return bizErrors.New(response.InvalidParam, "要移除的标签不存在")
		}
		return bizErrors.New(response.InternalError, "移除标签失败")
	}
	return nil
}

func (s *tagService) List(ctx context.Context, user *jwt.UserInfo) (*dto.TagListResponse, error) {
	tags, err := s.tagRepo.ListUniqueTagsByUserID(ctx, user.ID)
	if err != nil {
		return nil, bizErrors.New(response.InternalError, "获取标签列表失败")
	}
	return &dto.TagListResponse{Tags: tags}, nil
}
