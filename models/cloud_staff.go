// n9e-2kai: 负责人/人员表
// 简化的人员管理，只需要姓名，用于 RDS 负责人选择
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// CloudStaff 负责人/人员
type CloudStaff struct {
	Id   int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name string `json:"name" gorm:"type:varchar(64);uniqueIndex;not null"` // 姓名（唯一）

	// 元数据
	CreateBy  string `json:"create_by" gorm:"type:varchar(64)"`
	UpdateBy  string `json:"update_by" gorm:"type:varchar(64)"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (CloudStaff) TableName() string {
	return "cloud_staff"
}

// Add 添加人员
func (s *CloudStaff) Add(c *ctx.Context) error {
	now := time.Now().Unix()
	s.CreatedAt = now
	s.UpdatedAt = now
	return Insert(c, s)
}

// Update 更新人员
func (s *CloudStaff) Update(c *ctx.Context) error {
	s.UpdatedAt = time.Now().Unix()
	return DB(c).Model(s).Select("name", "update_by", "updated_at").Updates(s).Error
}

// CloudStaffGetAll 获取所有人员列表
func CloudStaffGetAll(c *ctx.Context, query string, limit, offset int) ([]CloudStaff, int64, error) {
	var list []CloudStaff
	var total int64

	session := DB(c).Model(&CloudStaff{})

	if query != "" {
		session = session.Where("name LIKE ?", "%"+query+"%")
	}

	session.Count(&total)

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Order("name ASC").Find(&list).Error
	return list, total, err
}

// CloudStaffGetByName 根据姓名获取人员
func CloudStaffGetByName(c *ctx.Context, name string) (*CloudStaff, error) {
	var staff CloudStaff
	err := DB(c).Where("name = ?", name).First(&staff).Error
	if err != nil {
		return nil, err
	}
	return &staff, nil
}

// CloudStaffGet 根据 ID 获取人员
func CloudStaffGet(c *ctx.Context, id int64) (*CloudStaff, error) {
	var staff CloudStaff
	err := DB(c).Where("id = ?", id).First(&staff).Error
	if err != nil {
		return nil, err
	}
	return &staff, nil
}

// CloudStaffDel 删除人员
func CloudStaffDel(c *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(c).Where("id IN ?", ids).Delete(&CloudStaff{}).Error
}

// CloudStaffNames 获取所有人员姓名列表（用于下拉选择）
func CloudStaffNames(c *ctx.Context) ([]string, error) {
	var names []string
	err := DB(c).Model(&CloudStaff{}).Pluck("name", &names).Error
	return names, err
}
