package database

import (
	"context"
	"errors"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openMySQL 连接已存在的数据库；连接成功后由 Open 按项目统一模型清单执行迁移。
func openMySQL(ctx context.Context, dsn string, businessNow func() time.Time) (*gorm.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("HT_MYSQL_DSN is required")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		return nil, errors.New("invalid MySQL DSN")
	}
	if cfg.DBName == "" {
		return nil, errors.New("HT_MYSQL_DSN must specify a database name")
	}
	cfg.ParseTime = true
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 3*time.Second, 3*time.Second, 3*time.Second
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       cfg.FormatDSN(),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		DisableAutomaticPing:   true,
		NowFunc:                businessNow,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, errors.New("open MySQL failed")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(8)
	pool.SetMaxIdleConns(4)
	pool.SetConnMaxLifetime(15 * time.Minute)
	if err = pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, errors.New("connect MySQL failed")
	}
	return db, nil
}
