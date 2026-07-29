// Package service 封装业务逻辑。
package service

import (
	"errors"
	"time"

	"github.com/kvm-inspection/Server/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Service 聚合所有业务方法
type Service struct {
	DB *gorm.DB
}

// New 创建
func New(db *gorm.DB) *Service { return &Service{DB: db} }

// ---------- Auth ----------

// Login 校验账号密码，返回用户
func (s *Service) Login(username, password string) (*model.User, error) {
	var u model.User
	if err := s.DB.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return &u, nil
}

// HashPassword 生成密码哈希
func HashPassword(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

// EnsureAdmin 若无任何用户，则创建默认 admin
func (s *Service) EnsureAdmin(defaultPwd string) error {
	var n int64
	if err := s.DB.Model(&model.User{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := HashPassword(defaultPwd)
	if err != nil {
		return err
	}
	return s.DB.Create(&model.User{Username: "admin", PasswordHash: hash, Role: "admin"}).Error
}

// ---------- Nodes ----------

// CreateNode 新增节点，返回完整对象（含 api_key 一次）
func (s *Service) CreateNode(name, ip, osName, virt string) (*model.Node, error) {
	n := &model.Node{Name: name, IP: ip, OS: osName, Virt: virt, Status: "offline"}
	if err := s.DB.Create(n).Error; err != nil {
		return nil, err
	}
	return n, nil
}

// ListNodes 列出所有节点
func (s *Service) ListNodes() ([]model.Node, error) {
	var out []model.Node
	err := s.DB.Order("created_at desc").Find(&out).Error
	return out, err
}

// GetNode 按 ID 查询
func (s *Service) GetNode(id string) (*model.Node, error) {
	var n model.Node
	err := s.DB.First(&n, "id = ?", id).Error
	return &n, err
}

// DeleteNode 删除
func (s *Service) DeleteNode(id string) error {
	return s.DB.Delete(&model.Node{}, "id = ?", id).Error
}

// AuthAgent 校验 nodeID + apiKey
func (s *Service) AuthAgent(nodeID, apiKey string) (*model.Node, error) {
	var n model.Node
	err := s.DB.First(&n, "id = ? AND api_key = ?", nodeID, apiKey).Error
	return &n, err
}

// UpdateHeartbeat 更新心跳
func (s *Service) UpdateHeartbeat(nodeID string, hostname string) error {
	now := time.Now()
	return s.DB.Model(&model.Node{}).Where("id = ?", nodeID).
		Updates(map[string]any{"status": "online", "last_heartbeat": now}).Error
}

// MarkOffline 标记离线
func (s *Service) MarkOffline(nodeID string) {
	s.DB.Model(&model.Node{}).Where("id = ?", nodeID).Update("status", "offline")
}

// ---------- Rules ----------

// ListRules 全部规则
func (s *Service) ListRules() ([]model.Rule, error) {
	var out []model.Rule
	err := s.DB.Order("id desc").Find(&out).Error
	return out, err
}

// CreateRule 新建规则
func (s *Service) CreateRule(r *model.Rule) error { return s.DB.Create(r).Error }

// UpdateRule 更新规则
func (s *Service) UpdateRule(r *model.Rule) error { return s.DB.Save(r).Error }

// DeleteRule 删除规则
func (s *Service) DeleteRule(id uint) error { return s.DB.Delete(&model.Rule{}, id).Error }

// ---------- Blacklist ----------

// ListBlacklist 全部
func (s *Service) ListBlacklist() ([]model.Blacklist, error) {
	var out []model.Blacklist
	err := s.DB.Order("id desc").Find(&out).Error
	return out, err
}

// AddBlacklist 新增
func (s *Service) AddBlacklist(b *model.Blacklist) error {
	if b.Status == "" {
		b.Status = "active"
	}
	if b.Action == "" {
		b.Action = "drop"
	}
	if b.Kind == "" {
		b.Kind = "ip"
	}
	return s.DB.Create(b).Error
}

// DeleteBlacklist 删除
func (s *Service) DeleteBlacklist(id uint) error { return s.DB.Delete(&model.Blacklist{}, id).Error }
