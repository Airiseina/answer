package connect

import (
	"answer/internal/model"
	"answer/pkg/logger"
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
	username := viper.GetString("mysql.user")
	password := viper.GetString("mysql.password")
	host := viper.GetString("mysql.host")
	port := viper.GetString("mysql.port")
	database := viper.GetString("mysql.name")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", username, password, host, port, database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error(" 数据库连接失败", zap.Error(err))
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("数据库指令失败", zap.Error(err))
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	err = db.AutoMigrate(&model.User{}, &model.Session{}, &model.Chat{}, &model.File{})
	if err != nil {
		logger.Error("数据库建表失败", zap.Error(err))
		return nil, err
	}
	return db, nil
}
