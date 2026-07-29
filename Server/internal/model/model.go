// Package model 定义 GORM 映射的 MySQL 表结构。
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Node 监控节点
type Node struct {
	ID            string         `gorm:"primaryKey;size:36" json:"id"`
	Name          string         `gorm:"size:128" json:"name"`
	IP            string         `gorm:"size:64" json:"ip"`
	OS            string         `gorm:"size:64" json:"os"`
	Virt          string         `gorm:"size:32" json:"virt"` // KVM / LXC
	APIKey        string         `gorm:"size:64;uniqueIndex" json:"-" `
	APIKeyMasked  string         `gorm:"-" json:"api_key"`
	Status        string         `gorm:"size:16" json:"status"` // online / offline
	LastHeartbeat *time.Time     `json:"last_heartbeat"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 生成 UUID 与 API Key
func (n *Node) BeforeCreate(_ *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	if n.APIKey == "" {
		n.APIKey = generateAPIKey()
	}
	n.APIKeyMasked = maskKey(n.APIKey)
	return nil
}

// AfterFind 回填掩码 key
func (n *Node) AfterFind(_ *gorm.DB) error {
	n.APIKeyMasked = maskKey(n.APIKey)
	return nil
}

func maskKey(k string) string {
	if len(k) <= 8 {
		return k
	}
	return k[:4] + "****" + k[len(k)-4:]
}

func generateAPIKey() string {
	return uuid.NewString() + uuid.NewString()[:8]
}

// User 巡查人员账号
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"size:64;uniqueIndex" json:"username"`
	PasswordHash string         `gorm:"size:128" json:"-"`
	Role         string         `gorm:"size:16" json:"role"` // admin / inspector
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Rule 违规规则
type Rule struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:128" json:"name"`
	Type      string         `gorm:"size:32" json:"type"` // blacklist_ip / blacklist_domain / port / keyword / protocol
	Pattern   string         `gorm:"type:text" json:"pattern"`
	Enabled   bool           `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Alert 告警
type Alert struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	NodeID    string         `gorm:"size:36;index" json:"node_id"`
	Type      string         `gorm:"size:32" json:"type"`
	Summary   string         `gorm:"size:512" json:"summary"`
	Status    string         `gorm:"size:16" json:"status"` // pending / handled
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	HandledAt *time.Time     `json:"handled_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Blacklist 黑名单
type Blacklist struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Target    string         `gorm:"size:255;index" json:"target"`
	Kind      string         `gorm:"size:16" json:"kind"`   // ip / domain
	Action    string         `gorm:"size:16" json:"action"` // drop / reject
	Status    string         `gorm:"size:16" json:"status"` // active / inactive
	NodeIDs   string         `gorm:"type:text" json:"-"`    // 逗号分隔节点ID；空表示全部
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// AllModels 返回需要 AutoMigrate 的全部模型
func AllModels() []any {
	return []any{&Node{}, &User{}, &Rule{}, &Alert{}, &Blacklist{}}
}
