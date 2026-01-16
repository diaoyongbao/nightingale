// n9e-2kai: RDS 实例负责人维护表
// 用于维护不同 RDS 实例的负责人信息，数据同步不影响该功能
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// CloudRDSOwner RDS 实例负责人
type CloudRDSOwner struct {
	Id int64 `json:"id" gorm:"primaryKey;autoIncrement"`

	// 关联信息
	InstanceId   string `json:"instance_id" gorm:"type:varchar(128);uniqueIndex;not null"` // RDS 实例 ID（唯一）
	InstanceName string `json:"instance_name" gorm:"type:varchar(256)"`                    // RDS 实例名称（冗余，方便查询）
	Provider     string `json:"provider" gorm:"type:varchar(32)"`                          // 云厂商

	// 负责人信息
	Owner      string `json:"owner" gorm:"type:varchar(128)"`       // 主负责人
	OwnerEmail string `json:"owner_email" gorm:"type:varchar(256)"` // 主负责人邮箱
	OwnerPhone string `json:"owner_phone" gorm:"type:varchar(32)"`  // 主负责人电话
	Backup     string `json:"backup" gorm:"type:varchar(128)"`      // 备份负责人
	Team       string `json:"team" gorm:"type:varchar(128)"`        // 所属团队
	Department string `json:"department" gorm:"type:varchar(128)"`  // 所属部门

	// 备注
	Note string `json:"note" gorm:"type:text"` // 备注信息

	// 元数据
	CreateBy  string `json:"create_by" gorm:"type:varchar(64)"`
	UpdateBy  string `json:"update_by" gorm:"type:varchar(64)"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (CloudRDSOwner) TableName() string {
	return "cloud_rds_owner"
}

// Add 添加负责人记录
func (o *CloudRDSOwner) Add(c *ctx.Context) error {
	now := time.Now().Unix()
	o.CreatedAt = now
	o.UpdatedAt = now
	return Insert(c, o)
}

// Update 更新负责人记录
func (o *CloudRDSOwner) Update(c *ctx.Context, cols ...string) error {
	o.UpdatedAt = time.Now().Unix()
	if len(cols) > 0 {
		cols = append(cols, "updated_at")
	}
	return DB(c).Model(o).Select(cols).Updates(o).Error
}

// CloudRDSOwnerGetByInstanceId 根据实例 ID 获取负责人
func CloudRDSOwnerGetByInstanceId(c *ctx.Context, instanceId string) (*CloudRDSOwner, error) {
	var owner CloudRDSOwner
	err := DB(c).Where("instance_id = ?", instanceId).First(&owner).Error
	if err != nil {
		return nil, err
	}
	return &owner, nil
}

// CloudRDSOwnerGet 根据 ID 获取负责人
func CloudRDSOwnerGet(c *ctx.Context, id int64) (*CloudRDSOwner, error) {
	var owner CloudRDSOwner
	err := DB(c).Where("id = ?", id).First(&owner).Error
	if err != nil {
		return nil, err
	}
	return &owner, nil
}

// CloudRDSOwnerGets 批量获取负责人列表
func CloudRDSOwnerGets(c *ctx.Context, instanceIds []string) ([]CloudRDSOwner, error) {
	if len(instanceIds) == 0 {
		return nil, nil
	}
	var owners []CloudRDSOwner
	err := DB(c).Where("instance_id IN ?", instanceIds).Find(&owners).Error
	return owners, err
}

// CloudRDSOwnerGetAll 获取所有负责人列表（支持分页和搜索）
func CloudRDSOwnerGetAll(c *ctx.Context, query string, limit, offset int) ([]CloudRDSOwner, int64, error) {
	var owners []CloudRDSOwner
	var total int64

	session := DB(c).Model(&CloudRDSOwner{})

	if query != "" {
		session = session.Where("instance_id LIKE ? OR instance_name LIKE ? OR owner LIKE ? OR team LIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	session.Count(&total)

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Order("updated_at DESC").Find(&owners).Error
	return owners, total, err
}

// CloudRDSOwnerUpsert 更新或插入负责人（根据 instance_id）
func CloudRDSOwnerUpsert(c *ctx.Context, owner *CloudRDSOwner) error {
	existing, err := CloudRDSOwnerGetByInstanceId(c, owner.InstanceId)
	if err != nil {
		// 不存在，插入
		return owner.Add(c)
	}
	// 存在，更新
	owner.Id = existing.Id
	owner.CreatedAt = existing.CreatedAt
	owner.CreateBy = existing.CreateBy
	return owner.Update(c, "instance_name", "provider", "owner", "owner_email", "owner_phone",
		"backup", "team", "department", "note", "update_by")
}

// CloudRDSOwnerDel 删除负责人记录
func CloudRDSOwnerDel(c *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(c).Where("id IN ?", ids).Delete(&CloudRDSOwner{}).Error
}

// CloudRDSOwnerDelByInstanceId 根据实例 ID 删除负责人
func CloudRDSOwnerDelByInstanceId(c *ctx.Context, instanceId string) error {
	return DB(c).Where("instance_id = ?", instanceId).Delete(&CloudRDSOwner{}).Error
}

// CloudRDSOwnerMap 获取实例 ID 到负责人的映射
func CloudRDSOwnerMap(c *ctx.Context, instanceIds []string) (map[string]*CloudRDSOwner, error) {
	owners, err := CloudRDSOwnerGets(c, instanceIds)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*CloudRDSOwner, len(owners))
	for i := range owners {
		result[owners[i].InstanceId] = &owners[i]
	}
	return result, nil
}
