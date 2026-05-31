package infra

import (
	"fmt"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectMysql(v *viper.Viper) (*gorm.DB, error) {
	username := v.GetString("mysql.user")
	password := v.GetString("mysql.password")
	host := v.GetString("mysql.host")
	port := v.GetString("mysql.port")
	database := v.GetString("mysql.name")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", username, password, host, port, database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sqlDB 失败: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	return db, nil
}

func ConnectPostgres(v *viper.Viper) (*gorm.DB, error) {
	host := v.GetString("postgres.host")
	port := v.GetString("postgres.port")
	user := v.GetString("postgres.user")
	password := v.GetString("postgres.password")
	dbname := v.GetString("postgres.dbname")
	sslmode := v.GetString("postgres.sslmode")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, dbname, sslmode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("PostgreSQL连接失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sqlDB 失败: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	return db, nil
}
