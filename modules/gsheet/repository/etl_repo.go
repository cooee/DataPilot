package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type EtlRepo struct {
	conn driver.Conn
}

func NewEtlRepo(conn driver.Conn) *EtlRepo {
	return &EtlRepo{conn: conn}
}

// RunMigrations 按文件名编号顺序执行 database/clickhouse/migrations/*.sql
func (r *EtlRepo) RunMigrations(ctx context.Context, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(migrationsDir, e.Name()))
		}
	}
	sort.Strings(files)

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		for _, stmt := range splitSQL(string(sql)) {
			if err := r.conn.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("exec %s: %w", f, err)
			}
		}
		fmt.Printf("[ch-migrate] applied: %s\n", filepath.Base(f))
	}
	return nil
}

// splitSQL 按分号+换行拆分多条语句，容忍尾部空白
func splitSQL(content string) []string {
	var stmts []string
	for _, s := range strings.Split(content, ";\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}

// ExecETL 读取 ETL SQL 文件，将 date_from / date_to 直接替换为字面量后执行。
// ClickHouse Go 驱动对 Date 类型参数支持不稳定，改为字符串替换更可靠。
func (r *EtlRepo) ExecETL(ctx context.Context, sqlFile string, params map[string]interface{}) error {
	raw, err := os.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("read etl sql %s: %w", sqlFile, err)
	}
	content := string(raw)
	// 将 {key:Date} 占位符替换为字面量日期字符串
	for k, v := range params {
		var literal string
		switch val := v.(type) {
		case time.Time:
			literal = "'" + val.Format("2006-01-02") + "'"
		default:
			literal = fmt.Sprintf("%v", val)
		}
		content = strings.ReplaceAll(content, "{"+k+":Date}", literal)
		content = strings.ReplaceAll(content, "{"+k+":DateTime}", literal)
		content = strings.ReplaceAll(content, "{"+k+":String}", "'"+fmt.Sprintf("%v", v)+"'")
	}
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	return r.conn.Exec(ctx2, content)
}

// DropPartition 删除指定分区（DWS 重算幂等用）
func (r *EtlRepo) DropPartition(ctx context.Context, table, partition string) error {
	return r.conn.Exec(ctx,
		fmt.Sprintf("ALTER TABLE %s DROP PARTITION '%s'", table, partition))
}
