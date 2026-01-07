package dbm

import (
	"context"
)

// DBClient 数据库客户端抽象接口（支持多种数据库类型扩展）
type DBClient interface {
	// Ping 检查连接
	Ping(ctx context.Context) error

	// Close 关闭连接
	Close() error

	// GetSessions 获取会话列表
	GetSessions(ctx context.Context, filter SessionFilter) ([]Session, error)

	// KillSession 杀死会话
	KillSession(ctx context.Context, sessionID int64) error

	// ExecuteSQL 执行 SQL
	ExecuteSQL(ctx context.Context, req SQLExecuteRequest) (*SQLExecuteResult, error)

	// CheckSQL 检查 SQL 语法
	CheckSQL(ctx context.Context, sql string) error

	// GetUncommittedTransactions 获取未提交事务
	GetUncommittedTransactions(ctx context.Context) ([]Transaction, error)

	// GetLockWaits 获取锁等待信息
	GetLockWaits(ctx context.Context) ([]LockWait, error)

	// GetInnoDBLocks 获取 InnoDB 锁信息
	GetInnoDBLocks(ctx context.Context) ([]InnoDBLock, error)

	// GetSlowQueries 获取慢查询（基于 Performance Schema）
	GetSlowQueries(ctx context.Context, filter SlowQueryFilter) ([]SlowQuery, error)

	// GetDatabases 获取数据库列表
	GetDatabases(ctx context.Context) ([]string, error)
}

// Session 会话信息
type Session struct {
	ID      int64  `json:"id"`
	User    string `json:"user"`
	Host    string `json:"host"`
	DB      string `json:"db"`
	Command string `json:"command"`
	Time    int    `json:"time"`
	State   string `json:"state"`
	Info    string `json:"info"`
}

// SessionFilter 会话过滤条件
type SessionFilter struct {
	Command string
	User    string
	DB      string
	MinTime int
}

// Transaction 事务信息
type Transaction struct {
	TrxID          string `json:"trx_id"`
	TrxState       string `json:"trx_state"`
	TrxStarted     string `json:"trx_started"`
	TrxLockID      string `json:"trx_requested_lock_id"`
	TrxWaitStarted string `json:"trx_wait_started"`
	TrxWeight      int64  `json:"trx_weight"`
	TrxThreadID    int64  `json:"trx_mysql_thread_id"`
	TrxQuery       string `json:"trx_query"`

	// Processlist 关联字段（用于哨兵匹配）
	User          string `json:"user"`
	Host          string `json:"host"`
	DB            string `json:"db"`
	RuntimeSec    int    `json:"runtime_sec"`
	ProcesslistID int64  `json:"processlist_id"` // 与 TrxThreadID 相同
	SQLText       string `json:"sql_text"`       // 与 TrxQuery 相同
}

// LockWait 锁等待信息
type LockWait struct {
	WaitingThreadID  int64  `json:"waiting_thread_id"`
	WaitingQuery     string `json:"waiting_query"`
	WaitingUser      string `json:"waiting_user"`
	WaitingHost      string `json:"waiting_host"`
	WaitingDB        string `json:"waiting_db"`
	WaitingTime      int    `json:"waiting_time"`
	BlockingThreadID int64  `json:"blocking_thread_id"`
	BlockingQuery    string `json:"blocking_query"`
	BlockingUser     string `json:"blocking_user"`
	BlockingHost     string `json:"blocking_host"`
	BlockingDB       string `json:"blocking_db"`
	BlockingTime     int    `json:"blocking_time"`
	LockType         string `json:"lock_type"`
	LockMode         string `json:"lock_mode"`
	LockTable        string `json:"lock_table"`
	LockIndex        string `json:"lock_index"`
	LockData         string `json:"lock_data"`
}

// InnoDBLock InnoDB 锁信息
type InnoDBLock struct {
	LockID    string `json:"lock_id"`
	LockTrxID string `json:"lock_trx_id"`
	LockMode  string `json:"lock_mode"`
	LockType  string `json:"lock_type"`
	LockTable string `json:"lock_table"`
	LockIndex string `json:"lock_index"`
	LockSpace int64  `json:"lock_space"`
	LockPage  int64  `json:"lock_page"`
	LockRec   int64  `json:"lock_rec"`
	LockData  string `json:"lock_data"`
	ThreadID  int64  `json:"thread_id"`
	User      string `json:"user"`
	Host      string `json:"host"`
	DB        string `json:"db"`
	Command   string `json:"command"`
	Time      int    `json:"time"`
	State     string `json:"state"`
	Query     string `json:"query"`
}

// SQLExecuteRequest SQL 执行请求
type SQLExecuteRequest struct {
	DB       string `json:"db_name"`
	SQL      string `json:"sql_content"`
	LimitNum int    `json:"limit_num"`
}

// SQLExecuteResult SQL 执行结果
type SQLExecuteResult struct {
	Rows         []map[string]interface{} `json:"rows"`
	ColumnList   []string                 `json:"column_list"`
	AffectedRows int64                    `json:"affected_rows"`
	FullSQL      string                   `json:"full_sql"`
	Status       int                      `json:"status"` // 0:success, 1:error
	Msg          string                   `json:"msg"`
	QueryTime    float64                  `json:"query_time,omitempty"`
	Error        string                   `json:"error,omitempty"`
}

// SlowQuery 慢查询信息
type SlowQuery struct {
	Checksum        string  `json:"checksum"`
	Fingerprint     string  `json:"fingerprint"`
	Sample          string  `json:"sample"`
	FirstSeen       string  `json:"first_seen"`
	LastSeen        string  `json:"last_seen"`
	ReviewedBy      string  `json:"reviewed_by"`
	ReviewedOn      string  `json:"reviewed_on"`
	Comments        string  `json:"comments"`
	QueryTimeAvg    float64 `json:"query_time_avg"`
	QueryTimeMax    float64 `json:"query_time_max"`
	QueryTimeMin    float64 `json:"query_time_min"`
	RowsExaminedAvg float64 `json:"rows_examined_avg"`
	RowsSentAvg     float64 `json:"rows_sent_avg"`
	TsCount         int64   `json:"ts_cnt"`
}

// SlowQueryFilter 慢查询过滤条件
type SlowQueryFilter struct {
	StartTime string
	EndTime   string
	DBName    string
	Limit     int
}
