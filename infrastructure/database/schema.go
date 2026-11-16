package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// migrateTables 按当前服务版本的模型迁移数据库，任一模型失败即停止启动。
func migrateTables(ctx context.Context, db *gorm.DB, models []any) error {
	// 同一数据库的服务依次迁移；锁属于连接，因此获取、迁移和释放必须使用同一条连接。
	return db.WithContext(ctx).Connection(func(connection *gorm.DB) (migrationErr error) {
		// 复用物理连接，不复用上一条查询的 SQL 和条件。
		connection = connection.Session(&gorm.Session{NewDB: true})
		var lockName string
		if err := connection.Raw("SELECT CONCAT('ht-schema:', SHA1(LOWER(DATABASE())))").Row().Scan(&lockName); err != nil {
			return fmt.Errorf("读取数据库迁移锁名称失败: %w", err)
		}
		var acquired sql.NullInt64
		if err := connection.Raw("SELECT GET_LOCK(?, 5)", lockName).Row().Scan(&acquired); err != nil {
			return fmt.Errorf("等待数据库迁移锁失败: %w", err)
		}
		if !acquired.Valid || acquired.Int64 != 1 {
			return errors.New("未取得数据库迁移锁，停止启动")
		}
		defer func() {
			// 启动超时后仍尝试释放；释放失败会使 Open 失败并关闭连接池。
			releaseContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var released sql.NullInt64
			releaseErr := connection.WithContext(releaseContext).Raw("SELECT RELEASE_LOCK(?)", lockName).Row().Scan(&released)
			if releaseErr != nil {
				migrationErr = errors.Join(migrationErr, fmt.Errorf("释放数据库迁移锁失败: %w", releaseErr))
			} else if !released.Valid || released.Int64 != 1 {
				migrationErr = errors.Join(migrationErr, errors.New("数据库迁移锁释放未确认"))
			}
		}()
		schemaDB := connection.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin")
		for _, tableModel := range models {
			if err := schemaDB.AutoMigrate(tableModel); err != nil {
				return fmt.Errorf("迁移数据表 %T 失败: %w", tableModel, err)
			}
		}
		return ctx.Err()
	})
}
