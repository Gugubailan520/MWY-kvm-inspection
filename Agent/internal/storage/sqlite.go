// Package storage 提供断网时的本地缓冲存储。
package storage

import (
	"context"
	"sync"
	"time"

	"github.com/kvm-inspection/common"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// eventRow 落表的事件行
type eventRow struct {
	ID        uint      `gorm:"primaryKey"`
	Payload   string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"index"`
}

// SQLite 缓冲存储
type SQLite struct {
	db *gorm.DB
	mu sync.Mutex
}

// NewSQLite 打开/迁移本地 SQLite
func NewSQLite(path string) (*SQLite, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&eventRow{}); err != nil {
		return nil, err
	}
	return &SQLite{db: db}, nil
}

// Push 将事件序列化入队
func (s *SQLite) Push(ctx context.Context, ev *common.NetworkEvent) error {
	payload, err := jsonMarshal(ev)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.WithContext(ctx).Create(&eventRow{Payload: payload, CreatedAt: ev.Timestamp}).Error
}

// Drain 批量取出最早的 limit 条，并在回调成功后删除。
func (s *SQLite) Drain(ctx context.Context, limit int, fn func(items []*common.NetworkEvent) error) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rows []eventRow
	if err := s.db.WithContext(ctx).Where("1=1").Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	items := make([]*common.NetworkEvent, 0, len(rows))
	for _, r := range rows {
		ev, err := jsonUnmarshal(r.Payload)
		if err == nil {
			items = append(items, ev)
		}
	}
	if err := fn(items); err != nil {
		return 0, err
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if err := s.db.WithContext(ctx).Delete(&eventRow{}, "id IN ?", ids).Error; err != nil {
		return len(items), err
	}
	return len(items), nil
}

// Count 当前缓冲条数
func (s *SQLite) Count(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&eventRow{}).Count(&n).Error
	return n, err
}

// Close 关闭
func (s *SQLite) Close() error {
	if sqlDB, err := s.db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}
