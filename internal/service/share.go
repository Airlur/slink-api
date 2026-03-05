package service

import (
	"context"

	"slink-api/internal/dto"
	"slink-api/internal/model"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/response"
	"slink-api/internal/repository"

	"gorm.io/gorm"
)

// DO to DTO Converter
func convertShareToDTO(m *model.Share) *dto.ShareResponse {
	return &dto.ShareResponse{
		ID:         m.ID,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
		ShortCode:  m.ShortCode,
		ShareTitle: m.ShareTitle,
		ShareDesc:  m.ShareDesc,
		ShareImage: m.ShareImage,
	}
}

type ShareService interface {
	Get(ctx context.Context, user *jwt.UserInfo, shortCode string) (*dto.GetShareInfoResponse, error)
	Upsert(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.UpdateShareInfoRequest) error
	// ===== 基础 CRUD 接口方法 =====
	Create(ctx context.Context, req *dto.CreateShareRequest) error
	Update(ctx context.Context, id uint, req *dto.UpdateShareRequest) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*dto.ShareResponse, error)
	List(ctx context.Context, offset, limit int) ([]*dto.ShareResponse, error)
	// ===== 根据索引查询接口方法 =====
	GetUniqueShortCode(ctx context.Context, shortCode string) (*dto.ShareResponse, error)
}

type shareService struct {
	db        *gorm.DB // 注入 gorm.DB 以便开启事务
	shareRepo repository.ShareRepository
	// 注入 shortlinkRepo 以便进行权限校验
	shortlinkRepo repository.ShortlinkRepository
}

func NewShareService(db *gorm.DB, shareRepo repository.ShareRepository, shortlinkRepo repository.ShortlinkRepository) ShareService {
	return &shareService{
		db:            db,
		shareRepo:     shareRepo,
		shortlinkRepo: shortlinkRepo,
	}
}

// Get 获取分享信息
func (s *shareService) Get(ctx context.Context, user *jwt.UserInfo, shortCode string) (*dto.GetShareInfoResponse, error) {
	// 1. 权限校验：确保用户有权访问这个短链接
	_, err := s.shortlinkRepo.GetUniqueShortCode(ctx, shortCode, false)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
		}
		return nil, bizErrors.New(response.InternalError, "数据库查询失败")
	}

	// 2. 获取分享信息
	share, err := s.shareRepo.GetByShortCode(ctx, shortCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 如果没有分享信息，这不是一个错误，返回一个空的响应即可
			return &dto.GetShareInfoResponse{ShortCode: shortCode}, nil
		}
		return nil, bizErrors.New(response.InternalError, "获取分享信息失败")
	}

	return &dto.GetShareInfoResponse{
		ShortCode:  share.ShortCode,
		ShareTitle: share.ShareTitle,
		ShareDesc:  share.ShareDesc,
		ShareImage: share.ShareImage,
	}, nil
}

// Upsert 创建或更更新分享信息
func (s *shareService) Upsert(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.UpdateShareInfoRequest) error {
	// 1. 权限校验：确保用户是该短链接的所有者
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

	// 2. 准备数据并执行 Upsert
	share := &model.Share{
		ShortCode: shortCode,
	}
	if req.ShareTitle != nil {
		share.ShareTitle = *req.ShareTitle
	}
	if req.ShareDesc != nil {
		share.ShareDesc = *req.ShareDesc
	}
	if req.ShareImage != nil {
		share.ShareImage = *req.ShareImage
	}

	return s.shareRepo.Upsert(ctx, share)
}

// ===== 基础 CRUD 接口方法实现 =====
func (s *shareService) Create(ctx context.Context, req *dto.CreateShareRequest) error {
	// DTO to DO
	share := &model.Share{
		ShortCode:  req.ShortCode,
		ShareTitle: req.ShareTitle,
		ShareDesc:  req.ShareDesc,
		ShareImage: req.ShareImage,
	}
	// TODO: 在这里添加创建前的业务逻辑校验，例如检查唯一索引是否存在

	return s.shareRepo.Create(ctx, share)
}

func (s *shareService) Update(ctx context.Context, id uint, req *dto.UpdateShareRequest) error {
	// 1. 检查记录是否存在
	_, err := s.shareRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return bizErrors.New(response.ShareNotFound, "Share 不存在")
		}
		return bizErrors.New(response.InternalError, "database error")
	}
	// 2. 构建更新 map
	// TODO: 根据你在 dto Update...Request 中的修改（改为指针），完善这里的逻辑
	// 示例:
	// updates := make(map[string]interface{})
	// if req.FieldName != nil {
	//     updates["FieldName"] = *req.FieldName
	// }
	updates := map[string]interface{}{
		"ShortCode":  req.ShortCode,
		"ShareTitle": req.ShareTitle,
		"ShareDesc":  req.ShareDesc,
		"ShareImage": req.ShareImage,
	}

	return s.shareRepo.Update(ctx, id, updates)
}

func (s *shareService) Delete(ctx context.Context, id uint) error {
	return s.shareRepo.Delete(ctx, id)
}

func (s *shareService) GetByID(ctx context.Context, id uint) (*dto.ShareResponse, error) {
	m, err := s.shareRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, bizErrors.New(response.ShareNotFound, "Share not found")
		}
		return nil, bizErrors.New(response.InternalError, "database error")
	}

	return convertShareToDTO(m), nil
}

func (s *shareService) List(ctx context.Context, offset, limit int) ([]*dto.ShareResponse, error) {
	items, err := s.shareRepo.List(ctx, offset, limit)
	if err != nil {
		return nil, bizErrors.New(response.InternalError, "database error")
	}
	var dtoList []*dto.ShareResponse
	for _, item := range items {
		dtoList = append(dtoList, convertShareToDTO(item))
	}

	return dtoList, nil
}

// ===== 根据索引查询接口方法实现 =====

func (s *shareService) GetUniqueShortCode(ctx context.Context, shortCode string) (*dto.ShareResponse, error) {
	// 1. 调用 Repository 层获取 DO
	m, err := s.shareRepo.GetUniqueShortCode(ctx, shortCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, bizErrors.New(response.ShareNotFound, "Share not found")
		}
		return nil, bizErrors.New(response.InternalError, "database error")
	}
	// 2. 将 DO 转换为 DTO 并返回
	return convertShareToDTO(m), nil
}
