package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"github.com/toolkits/pkg/str"
)

// DBASentinelRule DBA 哨兵规则
type DBASentinelRule struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(255);not null"`
	Description string `json:"description" gorm:"type:varchar(500)"`
	InstanceId  int64  `json:"instance_id" gorm:"not null;index"` // 关联 Archery 实例 ID

	// 启用状态
	Enabled bool `json:"enabled" gorm:"default:true"`

	// 规则类型: slow_query(慢查询), uncommitted_trx(未提交事务), lock_wait(锁等待)
	RuleType string `json:"rule_type" gorm:"type:varchar(50);not null"`

	// 规则配置
	MaxTime      int    `json:"max_time" gorm:"default:300"`            // 执行/未提交时长阈值(秒)
	MatchUser    string `json:"match_user" gorm:"type:varchar(255)"`    // 匹配用户 (正则)
	MatchDb      string `json:"match_db" gorm:"type:varchar(255)"`      // 匹配数据库 (正则)
	MatchSql     string `json:"match_sql" gorm:"type:text"`             // 匹配 SQL 关键词 (正则)
	MatchCommand string `json:"match_command" gorm:"type:varchar(100)"` // Query/Sleep 等
	MatchState   string `json:"match_state" gorm:"type:varchar(100)"`   // 匹配状态

	// 排除配置
	ExcludeUser string `json:"exclude_user" gorm:"type:varchar(255)"` // 排除用户 (正则)
	ExcludeDb   string `json:"exclude_db" gorm:"type:varchar(255)"`   // 排除数据库 (正则)
	ExcludeSql  string `json:"exclude_sql" gorm:"type:text"`          // 排除 SQL (正则)

	// 动作配置: kill(终止), notify_only(仅通知)
	Action string `json:"action" gorm:"type:varchar(50);default:'kill'"`

	// 通知配置 (JSON 数组)
	NotifyChannelIds   string `json:"notify_channel_ids" gorm:"type:varchar(500)"`    // 通知渠道 IDs
	NotifyUserGroupIds string `json:"notify_user_group_ids" gorm:"type:varchar(500)"` // 用户组 IDs

	// 执行间隔 (秒)
	CheckInterval int `json:"check_interval" gorm:"default:30"`

	// 上次检查时间
	LastCheckAt int64 `json:"last_check_at" gorm:"default:0"`

	// 元数据
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (DBASentinelRule) TableName() string {
	return "dba_sentinel_rule"
}

// 规则类型常量
const (
	SentinelRuleTypeSlowQuery      = "slow_query"
	SentinelRuleTypeUncommittedTrx = "uncommitted_trx"
	SentinelRuleTypeLockWait       = "lock_wait"
)

// 动作类型常量
const (
	SentinelActionKill       = "kill"
	SentinelActionNotifyOnly = "notify_only"
)

// Verify 验证规则配置
func (r *DBASentinelRule) Verify() error {
	if str.Dangerous(r.Name) {
		return errors.New("Name has invalid characters")
	}

	if r.Name == "" {
		return errors.New("Name is required")
	}

	if r.InstanceId <= 0 {
		return errors.New("InstanceId is required")
	}

	// 验证规则类型
	validRuleTypes := []string{SentinelRuleTypeSlowQuery, SentinelRuleTypeUncommittedTrx, SentinelRuleTypeLockWait}
	valid := false
	for _, t := range validRuleTypes {
		if r.RuleType == t {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("Invalid rule_type, must be one of: slow_query, uncommitted_trx, lock_wait")
	}

	// 验证动作类型
	if r.Action != SentinelActionKill && r.Action != SentinelActionNotifyOnly {
		return errors.New("Invalid action, must be one of: kill, notify_only")
	}

	// 验证正则表达式
	if r.MatchUser != "" {
		if _, err := regexp.Compile(r.MatchUser); err != nil {
			return errors.New("Invalid match_user regex pattern")
		}
	}
	if r.MatchDb != "" {
		if _, err := regexp.Compile(r.MatchDb); err != nil {
			return errors.New("Invalid match_db regex pattern")
		}
	}
	if r.MatchSql != "" {
		if _, err := regexp.Compile(r.MatchSql); err != nil {
			return errors.New("Invalid match_sql regex pattern")
		}
	}
	if r.ExcludeUser != "" {
		if _, err := regexp.Compile(r.ExcludeUser); err != nil {
			return errors.New("Invalid exclude_user regex pattern")
		}
	}
	if r.ExcludeDb != "" {
		if _, err := regexp.Compile(r.ExcludeDb); err != nil {
			return errors.New("Invalid exclude_db regex pattern")
		}
	}
	if r.ExcludeSql != "" {
		if _, err := regexp.Compile(r.ExcludeSql); err != nil {
			return errors.New("Invalid exclude_sql regex pattern")
		}
	}

	if r.MaxTime <= 0 {
		r.MaxTime = 300 // 默认 5 分钟
	}

	if r.CheckInterval <= 0 {
		r.CheckInterval = 30 // 默认 30 秒
	}

	return nil
}

// GetNotifyChannelIdList 获取通知渠道 ID 列表
func (r *DBASentinelRule) GetNotifyChannelIdList() []int64 {
	var ids []int64
	if r.NotifyChannelIds == "" {
		return ids
	}
	json.Unmarshal([]byte(r.NotifyChannelIds), &ids)
	return ids
}

// GetNotifyUserGroupIdList 获取用户组 ID 列表
func (r *DBASentinelRule) GetNotifyUserGroupIdList() []int64 {
	var ids []int64
	if r.NotifyUserGroupIds == "" {
		return ids
	}
	json.Unmarshal([]byte(r.NotifyUserGroupIds), &ids)
	return ids
}

// SetNotifyChannelIdList 设置通知渠道 ID 列表
func (r *DBASentinelRule) SetNotifyChannelIdList(ids []int64) {
	if len(ids) == 0 {
		r.NotifyChannelIds = ""
		return
	}
	b, _ := json.Marshal(ids)
	r.NotifyChannelIds = string(b)
}

// SetNotifyUserGroupIdList 设置用户组 ID 列表
func (r *DBASentinelRule) SetNotifyUserGroupIdList(ids []int64) {
	if len(ids) == 0 {
		r.NotifyUserGroupIds = ""
		return
	}
	b, _ := json.Marshal(ids)
	r.NotifyUserGroupIds = string(b)
}

// Add 添加规则
func (r *DBASentinelRule) Add(ctx *ctx.Context) error {
	if err := r.Verify(); err != nil {
		return err
	}

	now := time.Now().Unix()
	r.CreateAt = now
	r.UpdateAt = now
	return Insert(ctx, r)
}

// Update 更新规则
func (r *DBASentinelRule) Update(ctx *ctx.Context, selectFields ...string) error {
	if err := r.Verify(); err != nil {
		return err
	}

	r.UpdateAt = time.Now().Unix()
	return DB(ctx).Model(r).Select(selectFields).Updates(r).Error
}

// DBASentinelRuleGet 根据 ID 获取规则
func DBASentinelRuleGet(ctx *ctx.Context, id int64) (*DBASentinelRule, error) {
	var rule DBASentinelRule
	err := DB(ctx).Where("id = ?", id).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// DBASentinelRuleGetByWhere 根据条件获取规则
func DBASentinelRuleGetByWhere(ctx *ctx.Context, where string, args ...interface{}) (*DBASentinelRule, error) {
	var rule DBASentinelRule
	err := DB(ctx).Where(where, args...).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// DBASentinelRuleGets 获取规则列表
func DBASentinelRuleGets(ctx *ctx.Context, query string, limit, offset int) ([]DBASentinelRule, error) {
	var rules []DBASentinelRule
	session := DB(ctx).Order("id desc")

	if query != "" {
		session = session.Where("name like ? or description like ?", "%"+query+"%", "%"+query+"%")
	}

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Find(&rules).Error
	return rules, err
}

// DBASentinelRuleCount 获取规则总数
func DBASentinelRuleCount(ctx *ctx.Context, query string) (int64, error) {
	session := DB(ctx).Model(&DBASentinelRule{})

	if query != "" {
		session = session.Where("name like ? or description like ?", "%"+query+"%", "%"+query+"%")
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// DBASentinelRuleGetsByInstanceId 根据实例 ID 获取规则列表
func DBASentinelRuleGetsByInstanceId(ctx *ctx.Context, instanceId int64) ([]DBASentinelRule, error) {
	var rules []DBASentinelRule
	err := DB(ctx).Where("instance_id = ? AND enabled = ?", instanceId, true).Find(&rules).Error
	return rules, err
}

// DBASentinelRuleGetsEnabled 获取所有启用的规则
func DBASentinelRuleGetsEnabled(ctx *ctx.Context) ([]DBASentinelRule, error) {
	var rules []DBASentinelRule
	err := DB(ctx).Where("enabled = ?", true).Find(&rules).Error
	return rules, err
}

// DBASentinelRuleDel 删除规则
func DBASentinelRuleDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(ctx).Where("id in ?", ids).Delete(new(DBASentinelRule)).Error
}

// UpdateLastCheckAt 更新上次检查时间
func (r *DBASentinelRule) UpdateLastCheckAt(ctx *ctx.Context) error {
	return DB(ctx).Model(r).Update("last_check_at", time.Now().Unix()).Error
}

// ShouldCheck 是否应该执行检查
func (r *DBASentinelRule) ShouldCheck() bool {
	if !r.Enabled {
		return false
	}
	now := time.Now().Unix()
	return now-r.LastCheckAt >= int64(r.CheckInterval)
}

// MatchSession 检查会话是否匹配规则
func (r *DBASentinelRule) MatchSession(user, db, command, state, info string, execTime int) (bool, string) {
	// 检查执行时间
	if execTime < r.MaxTime {
		return false, ""
	}

	// 检查排除规则
	if r.ExcludeUser != "" {
		if matched, _ := regexp.MatchString(r.ExcludeUser, user); matched {
			return false, ""
		}
	}
	if r.ExcludeDb != "" && db != "" {
		if matched, _ := regexp.MatchString(r.ExcludeDb, db); matched {
			return false, ""
		}
	}
	if r.ExcludeSql != "" && info != "" {
		if matched, _ := regexp.MatchString(r.ExcludeSql, info); matched {
			return false, ""
		}
	}

	// 检查匹配规则
	if r.MatchUser != "" {
		if matched, _ := regexp.MatchString(r.MatchUser, user); !matched {
			return false, ""
		}
	}
	if r.MatchDb != "" && db != "" {
		if matched, _ := regexp.MatchString(r.MatchDb, db); !matched {
			return false, ""
		}
	}
	if r.MatchCommand != "" && command != "" {
		if matched, _ := regexp.MatchString(r.MatchCommand, command); !matched {
			return false, ""
		}
	}
	if r.MatchState != "" && state != "" {
		if matched, _ := regexp.MatchString(r.MatchState, state); !matched {
			return false, ""
		}
	}
	if r.MatchSql != "" && info != "" {
		if matched, _ := regexp.MatchString(r.MatchSql, info); !matched {
			return false, ""
		}
	}

	reason := fmt.Sprintf("规则[%s]: 执行时间 %ds 超过阈值 %ds", r.Name, execTime, r.MaxTime)
	return true, reason
}
