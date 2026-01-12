// Package models AI 会话归档模型
// n9e-2kai: AI 助手模块 - 会话归档模型
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// AISessionArchive AI 助手会话归档表
type AISessionArchive struct {
	Id             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	SessionId      string `json:"session_id" gorm:"type:varchar(64);not null;index"`
	UserId         int64  `json:"user_id" gorm:"not null;index"`
	Mode           string `json:"mode" gorm:"type:varchar(32)"`
	MessageCount   int    `json:"message_count" gorm:"default:0"`
	FirstMessageAt int64  `json:"first_message_at"`
	LastMessageAt  int64  `json:"last_message_at"`
	Messages       string `json:"messages" gorm:"type:longtext"` // JSON 数组（脱敏后）
	TraceIds       string `json:"trace_ids" gorm:"type:text"`    // JSON 数组
	ArchivedAt     int64  `json:"archived_at" gorm:"not null;index"`
	ArchivedBy     string `json:"archived_by" gorm:"type:varchar(64)"`
	ArchiveReason  string `json:"archive_reason" gorm:"type:varchar(128)"` // manual/auto_expired/user_deleted
}

func (AISessionArchive) TableName() string {
	return "ai_session_archive"
}

// 归档原因常量
const (
	ArchiveReasonManual      = "manual"
	ArchiveReasonAutoExpired = "auto_expired"
	ArchiveReasonUserDeleted = "user_deleted"
)

// Create 创建归档记录
func (a *AISessionArchive) Create(c *ctx.Context) error {
	if a.ArchivedAt == 0 {
		a.ArchivedAt = time.Now().Unix()
	}
	return DB(c).Create(a).Error
}

// AISessionArchiveGets 查询归档列表
func AISessionArchiveGets(c *ctx.Context, where string, args ...interface{}) ([]AISessionArchive, error) {
	var archives []AISessionArchive
	session := DB(c).Order("archived_at desc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&archives).Error
	return archives, err
}

// AISessionArchiveGetBySessionId 根据 session_id 查询
func AISessionArchiveGetBySessionId(c *ctx.Context, sessionId string) (*AISessionArchive, error) {
	var archive AISessionArchive
	err := DB(c).Where("session_id = ?", sessionId).First(&archive).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &archive, err
}

// AISessionArchiveGetsByUserId 根据用户 ID 查询
func AISessionArchiveGetsByUserId(c *ctx.Context, userId int64, limit int) ([]AISessionArchive, error) {
	var archives []AISessionArchive
	session := DB(c).Where("user_id = ?", userId).Order("archived_at desc")
	if limit > 0 {
		session = session.Limit(limit)
	}
	err := session.Find(&archives).Error
	return archives, err
}

// AISessionArchiveCount 统计归档数量
func AISessionArchiveCount(c *ctx.Context, where string, args ...interface{}) (int64, error) {
	var count int64
	session := DB(c).Model(&AISessionArchive{})
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Count(&count).Error
	return count, err
}

// Delete 删除归档记录
func (a *AISessionArchive) Delete(c *ctx.Context) error {
	return DB(c).Delete(a).Error
}

// AISessionArchiveCleanup 清理过期归档（保留指定天数）
func AISessionArchiveCleanup(c *ctx.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Unix()
	result := DB(c).Where("archived_at < ?", cutoff).Delete(&AISessionArchive{})
	return result.RowsAffected, result.Error
}
