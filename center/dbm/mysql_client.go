package dbm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DBInstanceInfo 数据库实例信息（避免循环依赖）
type DBInstanceInfo struct {
	Id             int64
	InstanceName   string
	Host           string
	Port           int
	Username       string
	Password       string
	Charset        string
	MaxConnections int
	MaxIdleConns   int
}

// MySQLClient MySQL 客户端实现
type MySQLClient struct {
	instanceID int64
	db         *sql.DB
}

// NewMySQLClient 创建 MySQL 客户端
func NewMySQLClient(instanceID int64, dsn string, maxOpenConns, maxIdleConns int) (*MySQLClient, error) {
	connManager := GetGlobalConnectionManager()
	db, err := connManager.GetConnection(instanceID, dsn, maxOpenConns, maxIdleConns)
	if err != nil {
		return nil, err
	}

	return &MySQLClient{
		instanceID: instanceID,
		db:         db,
	}, nil
}

// Ping 检查连接
func (c *MySQLClient) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// Close 关闭连接（不关闭连接池，只是标记客户端无效）
func (c *MySQLClient) Close() error {
	// 连接池由 ConnectionManager 管理，这里不需要关闭
	return nil
}

// GetSessions 获取会话列表
func (c *MySQLClient) GetSessions(ctx context.Context, filter SessionFilter) ([]Session, error) {
	query := `
		SELECT 
			ID, 
			USER, 
			HOST, 
			IFNULL(DB, '') as DB,
			COMMAND,
			TIME,
			IFNULL(STATE, '') as STATE,
			IFNULL(INFO, '') as INFO
		FROM information_schema.PROCESSLIST
		WHERE 1=1
	`
	args := []interface{}{}

	if filter.Command != "" {
		query += " AND COMMAND = ?"
		args = append(args, filter.Command)
	}

	if filter.User != "" {
		query += " AND USER = ?"
		args = append(args, filter.User)
	}

	if filter.DB != "" {
		query += " AND DB = ?"
		args = append(args, filter.DB)
	}

	if filter.MinTime > 0 {
		query += " AND TIME >= ?"
		args = append(args, filter.MinTime)
	}

	query += " ORDER BY TIME DESC"

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.User, &s.Host, &s.DB, &s.Command, &s.Time, &s.State, &s.Info); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// KillSession 杀死会话
func (c *MySQLClient) KillSession(ctx context.Context, sessionID int64) error {
	_, err := c.db.ExecContext(ctx, "KILL ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to kill session %d: %w", sessionID, err)
	}
	return nil
}

// ExecuteSQL 执行 SQL
func (c *MySQLClient) ExecuteSQL(ctx context.Context, req SQLExecuteRequest) (*SQLExecuteResult, error) {
	result := &SQLExecuteResult{
		FullSQL: req.SQL,
		Status:  0,
	}

	start := time.Now()

	// 校验 SQL 不能为空
	trimmedSQL := strings.TrimSpace(req.SQL)
	if trimmedSQL == "" {
		result.Status = 1
		result.Error = "SQL statement cannot be empty"
		return result, nil
	}

	// 切换数据库
	if req.DB != "" {
		if _, err := c.db.ExecContext(ctx, "USE "+req.DB); err != nil {
			result.Status = 1
			result.Error = fmt.Sprintf("Failed to use database %s: %v", req.DB, err)
			return result, nil
		}
	}

	// 检测 SQL 类型
	sqlUpper := strings.ToUpper(trimmedSQL)
	isSelect := strings.HasPrefix(sqlUpper, "SELECT") || strings.HasPrefix(sqlUpper, "SHOW") || strings.HasPrefix(sqlUpper, "DESC")

	if isSelect {
		// 查询语句
		return c.executeQuery(ctx, req, result, start)
	} else {
		// 修改语句
		return c.executeNonQuery(ctx, req, result, start)
	}
}

// executeQuery 执行查询语句
func (c *MySQLClient) executeQuery(ctx context.Context, req SQLExecuteRequest, result *SQLExecuteResult, start time.Time) (*SQLExecuteResult, error) {
	// 使用 trimmed SQL
	sql := strings.TrimSpace(req.SQL)
	sqlUpper := strings.ToUpper(sql)

	// 只对 SELECT 语句自动添加 LIMIT（SHOW/DESC/EXPLAIN 等不支持 LIMIT）
	isSelectQuery := strings.HasPrefix(sqlUpper, "SELECT")
	if isSelectQuery && req.LimitNum > 0 && !strings.Contains(sqlUpper, "LIMIT") {
		sql = fmt.Sprintf("%s LIMIT %d", sql, req.LimitNum)
		result.FullSQL = sql
	}

	rows, err := c.db.QueryContext(ctx, sql)
	if err != nil {
		result.Status = 1
		result.Error = err.Error()
		result.QueryTime = time.Since(start).Seconds()
		return result, nil
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		result.Status = 1
		result.Error = err.Error()
		result.QueryTime = time.Since(start).Seconds()
		return result, nil
	}
	result.ColumnList = columns

	// 读取数据
	result.Rows = make([]map[string]interface{}, 0)
	for rows.Next() {
		// 创建扫描目标
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			result.Status = 1
			result.Error = err.Error()
			break
		}

		// 构造行数据
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// 处理 []byte 类型
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		result.Rows = append(result.Rows, row)
	}

	result.AffectedRows = int64(len(result.Rows))
	result.QueryTime = time.Since(start).Seconds()
	result.Msg = fmt.Sprintf("Query OK, %d rows returned", result.AffectedRows)

	return result, nil
}

// executeNonQuery 执行非查询语句
func (c *MySQLClient) executeNonQuery(ctx context.Context, req SQLExecuteRequest, result *SQLExecuteResult, start time.Time) (*SQLExecuteResult, error) {
	execResult, err := c.db.ExecContext(ctx, req.SQL)
	if err != nil {
		result.Status = 1
		result.Error = err.Error()
		result.QueryTime = time.Since(start).Seconds()
		return result, nil
	}

	affected, _ := execResult.RowsAffected()
	result.AffectedRows = affected
	result.QueryTime = time.Since(start).Seconds()
	result.Msg = fmt.Sprintf("Query OK, %d rows affected", affected)

	return result, nil
}

// CheckSQL 检查 SQL 语法
func (c *MySQLClient) CheckSQL(ctx context.Context, sql string) error {
	// 使用 EXPLAIN 检查语法
	_, err := c.db.QueryContext(ctx, "EXPLAIN "+sql)
	return err
}

// GetUncommittedTransactions 获取未提交事务
func (c *MySQLClient) GetUncommittedTransactions(ctx context.Context) ([]Transaction, error) {
	// JOIN processlist 获取 User, Host, DB 等信息（哨兵需要）
	query := `
		SELECT 
			t.trx_id,
			t.trx_state,
			t.trx_started,
			IFNULL(t.trx_requested_lock_id, '') as trx_requested_lock_id,
			IFNULL(t.trx_wait_started, '') as trx_wait_started,
			t.trx_weight,
			t.trx_mysql_thread_id,
			IFNULL(t.trx_query, '') as trx_query,
			IFNULL(p.USER, '') as user,
			IFNULL(p.HOST, '') as host,
			IFNULL(p.DB, '') as db,
			TIMESTAMPDIFF(SECOND, t.trx_started, NOW()) as runtime_sec
		FROM information_schema.INNODB_TRX t
		LEFT JOIN information_schema.PROCESSLIST p ON t.trx_mysql_thread_id = p.ID
		ORDER BY t.trx_started ASC
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(
			&t.TrxID, &t.TrxState, &t.TrxStarted, &t.TrxLockID,
			&t.TrxWaitStarted, &t.TrxWeight, &t.TrxThreadID, &t.TrxQuery,
			&t.User, &t.Host, &t.DB, &t.RuntimeSec,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// 填充别名字段
		t.ProcesslistID = t.TrxThreadID
		t.SQLText = t.TrxQuery

		transactions = append(transactions, t)
	}

	return transactions, rows.Err()
}

// GetLockWaits 获取锁等待信息（MySQL 5.7+）
func (c *MySQLClient) GetLockWaits(ctx context.Context) ([]LockWait, error) {
	// MySQL 8.0+ 使用 performance_schema.data_lock_waits
	// MySQL 5.7 使用 sys.innodb_lock_waits 视图
	query := `
		SELECT 
			waiting_pid as waiting_thread_id,
			IFNULL(waiting_query, '') as waiting_query,
			waiting_account as waiting_user,
			'' as waiting_host,
			IFNULL(waiting_lock_mode, '') as lock_mode,
			blocking_pid as blocking_thread_id,
			IFNULL(blocking_query, '') as blocking_query,
			blocking_account as blocking_user,
			'' as blocking_host,
			TIMESTAMPDIFF(SECOND, wait_started, NOW()) as waiting_time,
			0 as blocking_time,
			IFNULL(locked_table, '') as lock_table,
			'' as lock_type,
			IFNULL(locked_index, '') as lock_index,
			'' as lock_data
		FROM sys.innodb_lock_waits
		WHERE 1=1
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		// 如果sys schema不存在，返回空列表
		return []LockWait{}, nil
	}
	defer rows.Close()

	var lockWaits []LockWait
	for rows.Next() {
		var lw LockWait
		if err := rows.Scan(
			&lw.WaitingThreadID, &lw.WaitingQuery, &lw.WaitingUser, &lw.WaitingHost,
			&lw.LockMode, &lw.BlockingThreadID, &lw.BlockingQuery, &lw.BlockingUser,
			&lw.BlockingHost, &lw.WaitingTime, &lw.BlockingTime, &lw.LockTable,
			&lw.LockType, &lw.LockIndex, &lw.LockData,
		); err != nil {
			return nil, fmt.Errorf("failed to scan lock wait: %w", err)
		}
		lockWaits = append(lockWaits, lw)
	}

	return lockWaits, rows.Err()
}

// GetInnoDBLocks 获取 InnoDB 锁信息
func (c *MySQLClient) GetInnoDBLocks(ctx context.Context) ([]InnoDBLock, error) {
	// 简化版本，仅查询基本信息
	query := `
		SELECT 
			t.trx_id as lock_trx_id,
			t.trx_mysql_thread_id as thread_id,
			IFNULL(p.USER, '') as user,
			IFNULL(p.HOST, '') as host,
			IFNULL(p.DB, '') as db,
			IFNULL(p.COMMAND, '') as command,
			IFNULL(p.TIME, 0) as time,
			IFNULL(p.STATE, '') as state,
			IFNULL(t.trx_query, '') as query,
			'' as lock_id,
			'' as lock_mode,
			'' as lock_type,
			'' as lock_table,
			'' as lock_index,
			0 as lock_space,
			0 as lock_page,
			0 as lock_rec,
			'' as lock_data
		FROM information_schema.INNODB_TRX t
		LEFT JOIN information_schema.PROCESSLIST p ON t.trx_mysql_thread_id = p.ID
		ORDER BY t.trx_started ASC
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query innodb locks: %w", err)
	}
	defer rows.Close()

	var locks []InnoDBLock
	for rows.Next() {
		var lock InnoDBLock
		if err := rows.Scan(
			&lock.LockTrxID, &lock.ThreadID, &lock.User, &lock.Host, &lock.DB,
			&lock.Command, &lock.Time, &lock.State, &lock.Query, &lock.LockID,
			&lock.LockMode, &lock.LockType, &lock.LockTable, &lock.LockIndex,
			&lock.LockSpace, &lock.LockPage, &lock.LockRec, &lock.LockData,
		); err != nil {
			return nil, fmt.Errorf("failed to scan innodb lock: %w", err)
		}
		locks = append(locks, lock)
	}

	return locks, rows.Err()
}

// GetSlowQueries 获取慢查询（基于 Performance Schema）
func (c *MySQLClient) GetSlowQueries(ctx context.Context, filter SlowQueryFilter) ([]SlowQuery, error) {
	query := `
		SELECT 
			IFNULL(DIGEST, '') as checksum,
			IFNULL(DIGEST_TEXT, '') as fingerprint,
			IFNULL(QUERY_SAMPLE_TEXT, '') as sample,
			IFNULL(FIRST_SEEN, '') as first_seen,
			IFNULL(LAST_SEEN, '') as last_seen,
			'' as reviewed_by,
			'' as reviewed_on,
			'' as comments,
			AVG_TIMER_WAIT / 1000000000000 as query_time_avg,
			MAX_TIMER_WAIT / 1000000000000 as query_time_max,
			MIN_TIMER_WAIT / 1000000000000 as query_time_min,
			IFNULL(SUM_ROWS_EXAMINED / COUNT_STAR, 0) as rows_examined_avg,
			IFNULL(SUM_ROWS_SENT / COUNT_STAR, 0) as rows_sent_avg,
			COUNT_STAR as ts_cnt
		FROM performance_schema.events_statements_summary_by_digest
		WHERE SCHEMA_NAME IS NOT NULL
	`
	args := []interface{}{}

	if filter.DBName != "" {
		query += " AND SCHEMA_NAME = ?"
		args = append(args, filter.DBName)
	}

	query += " ORDER BY AVG_TIMER_WAIT DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	} else {
		query += " LIMIT 100"
	}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query slow queries: %w", err)
	}
	defer rows.Close()

	var slowQueries []SlowQuery
	for rows.Next() {
		var sq SlowQuery
		if err := rows.Scan(
			&sq.Checksum, &sq.Fingerprint, &sq.Sample, &sq.FirstSeen, &sq.LastSeen,
			&sq.ReviewedBy, &sq.ReviewedOn, &sq.Comments, &sq.QueryTimeAvg,
			&sq.QueryTimeMax, &sq.QueryTimeMin, &sq.RowsExaminedAvg, &sq.RowsSentAvg, &sq.TsCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan slow query: %w", err)
		}
		slowQueries = append(slowQueries, sq)
	}

	return slowQueries, rows.Err()
}

// GetDatabases 获取数据库列表
func (c *MySQLClient) GetDatabases(ctx context.Context) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("failed to get databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database: %w", err)
		}
		// 过滤系统数据库
		if dbName != "information_schema" && dbName != "performance_schema" &&
			dbName != "mysql" && dbName != "sys" {
			databases = append(databases, dbName)
		}
	}

	return databases, rows.Err()
}

// GetTables 获取指定数据库的表列表
func (c *MySQLClient) GetTables(ctx context.Context, dbName string) ([]TableInfo, error) {
	query := `
		SELECT 
			TABLE_NAME,
			IFNULL(TABLE_COMMENT, '') as TABLE_COMMENT,
			IFNULL(ENGINE, '') as ENGINE,
			IFNULL(TABLE_ROWS, 0) as TABLE_ROWS,
			IFNULL(DATA_LENGTH, 0) as DATA_LENGTH,
			IFNULL(CREATE_TIME, '') as CREATE_TIME
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`

	rows, err := c.db.QueryContext(ctx, query, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Comment, &t.Engine, &t.Rows, &t.DataLength, &t.CreateTime); err != nil {
			return nil, fmt.Errorf("failed to scan table: %w", err)
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// GetTableColumns 获取指定表的列信息
func (c *MySQLClient) GetTableColumns(ctx context.Context, dbName, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT 
			COLUMN_NAME,
			COLUMN_TYPE,
			IS_NULLABLE,
			IFNULL(COLUMN_KEY, '') as COLUMN_KEY,
			IFNULL(COLUMN_DEFAULT, '') as COLUMN_DEFAULT,
			IFNULL(EXTRA, '') as EXTRA,
			IFNULL(COLUMN_COMMENT, '') as COLUMN_COMMENT,
			ORDINAL_POSITION
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := c.db.QueryContext(ctx, query, dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable string
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &col.Key, &col.Default, &col.Extra, &col.Comment, &col.OrdinalPos); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}
		col.Nullable = (nullable == "YES")
		columns = append(columns, col)
	}

	return columns, rows.Err()
}
