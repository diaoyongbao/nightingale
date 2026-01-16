// n9e-2kai: SQL 指纹生成工具函数
// 注意: cloud_rds_slowlog 表已废弃，新架构使用：
// - cloud_rds_slowlog_detail: 存储原始明细（7天清理）
// - cloud_rds_slowlog_report: 存储聚合报表（90天保留）
package models

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strings"
)

// GenerateSQLFingerprint 生成 SQL 指纹（将参数替换为占位符）
func GenerateSQLFingerprint(sql string) string {
	if sql == "" {
		return ""
	}

	fingerprint := sql

	// 1. 移除注释
	// 移除多行注释 /* ... */
	reMultiLineComment := regexp.MustCompile(`/\*.*?\*/`)
	fingerprint = reMultiLineComment.ReplaceAllString(fingerprint, "")
	// 移除单行注释 -- ...
	reSingleLineComment := regexp.MustCompile(`--.*$`)
	fingerprint = reSingleLineComment.ReplaceAllString(fingerprint, "")

	// 2. 替换字符串常量 'xxx' 或 "xxx" 为 ?
	reString := regexp.MustCompile(`'[^']*'|"[^"]*"`)
	fingerprint = reString.ReplaceAllString(fingerprint, "?")

	// 3. 替换数字为 ?
	reNumber := regexp.MustCompile(`\b\d+\.?\d*\b`)
	fingerprint = reNumber.ReplaceAllString(fingerprint, "?")

	// 4. 替换 IN 列表 IN (?, ?, ?) 为 IN (?)
	reInList := regexp.MustCompile(`IN\s*\(\s*\?(?:\s*,\s*\?)*\s*\)`)
	fingerprint = reInList.ReplaceAllString(fingerprint, "IN (?)")

	// 5. 替换多个空白字符为单个空格
	reWhitespace := regexp.MustCompile(`\s+`)
	fingerprint = reWhitespace.ReplaceAllString(fingerprint, " ")

	// 6. 去除首尾空白
	fingerprint = strings.TrimSpace(fingerprint)

	// 7. 转换为大写以便统一比较
	fingerprint = strings.ToUpper(fingerprint)

	return fingerprint
}

// GenerateSQLHash 生成 SQL 指纹的 MD5 哈希
func GenerateSQLHash(fingerprint string) string {
	if fingerprint == "" {
		return ""
	}
	hash := md5.Sum([]byte(fingerprint))
	return hex.EncodeToString(hash[:])
}
