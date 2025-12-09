// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package dbmysql 提供基于 MySQL 的数据库连接和 GORM ORM 操作功能。
package dbmysql

import (
	"context"
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/rs/zerolog"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
	"sync"
	"time"
)

// GormLoggerAdapter 适配器，将框架日志器适配到 GORM 日志接口
type GormLoggerAdapter struct {
	logger bootstrap.LoggerWrapper
	AppCtx fiberhouse.IContext
}

func (l *GormLoggerAdapter) Printf(format string, v ...interface{}) {
	// 格式化日志消息
	msg := fmt.Sprintf(format, v...)
	logOriDefault := appconfig.LogOrigin("Mysql")
	if l.AppCtx != nil {
		if l.AppCtx.GetConfig().LogOriginMysql().String() != "" {
			logOriDefault = l.AppCtx.GetConfig().LogOriginMysql()
		}
	}
	switch {
	case strings.Contains(format, "[error]"):
		l.logger.ErrorWith(logOriDefault).Msg(msg)
	case strings.Contains(format, "[warn]"):
		l.logger.WarnWith(logOriDefault).Msg(msg)
	case strings.Contains(format, "[info]"):
		l.logger.Info(logOriDefault).Msg(msg)
	default:
		if l.logger.GetLevel() <= zerolog.DebugLevel {
			l.logger.Debug(logOriDefault).Msg(msg)
		} else if l.logger.GetLevel() <= zerolog.InfoLevel {
			l.logger.Info(logOriDefault).Msg(msg)
		} else if l.logger.GetLevel() <= zerolog.WarnLevel {
			l.logger.Warn(logOriDefault).Msg(msg)
		} else if l.logger.GetLevel() <= zerolog.ErrorLevel {
			l.logger.Error(logOriDefault).Msg(msg)
		} else if l.logger.GetLevel() <= zerolog.FatalLevel {
			l.logger.Fatal(logOriDefault).Msg(msg)
		} else {
			l.logger.Panic(logOriDefault).Msg(msg)
		}
	}
}

// MysqlDb mysql数据库操作封装
type MysqlDb struct {
	Client       *gorm.DB
	Ctx          fiberhouse.IContext
	lock         *sync.RWMutex
	confPathname string
}

func NewMysqlDb(appCtx fiberhouse.IContext, confPath ...string) (*MysqlDb, error) {
	client, err := NewClient(appCtx, confPath...)
	if err != nil {
		return nil, err
	}
	db := &MysqlDb{
		Client:       client,
		Ctx:          appCtx,
		lock:         &sync.RWMutex{},
		confPathname: constant.DefaultMysqlDBConfName,
	}
	if len(confPath) > 0 {
		db.confPathname = confPath[0]
	}
	return db, nil
}

// NewClient 创建并返回一个新的 GORM 数据库连接
func NewClient(appCtx fiberhouse.IContext, confPath ...string) (*gorm.DB, error) {
	var basePath string
	if len(confPath) > 0 && confPath[0] != "" {
		basePath = confPath[0]
	} else {
		basePath = constant.DefaultMysqlDBConfName
	}

	// 读取配置
	aConf := appCtx.GetConfig()
	var (
		dsn             = aConf.String(basePath + ".dsn")
		maxIdleConns    = aConf.Int(basePath + ".gorm.maxIdleConns")
		maxOpenConns    = aConf.Int(basePath + ".gorm.maxOpenConns")
		connMaxLifetime = aConf.Duration(basePath+".gorm.connMaxLifetime") * time.Second
		connMaxIdleTime = aConf.Duration(basePath+".gorm.connMaxIdleTime") * time.Second
		// 日志器配置
		logLevel          = aConf.String(basePath + ".gorm.logger.level")
		slowThreshold     = aConf.Duration(basePath+".gorm.logger.slowThreshold") * time.Millisecond
		colorful          = aConf.Bool(basePath + ".gorm.logger.colorful")
		enableLogger      = aConf.Bool(basePath + ".gorm.logger.enable")
		skipDefaultFields = aConf.Bool(basePath + ".gorm.logger.skipDefaultFields")
	)

	// 验证必要配置
	if dsn == "" {
		err := fmt.Errorf("mysql dsn is required in config path: %s.dsn", basePath)
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginMysql()).Err(err).Msg("mysql dsn configuration missing")
		return nil, err
	}

	// 配置 GORM 日志器
	var gormLogger logger.Interface
	if enableLogger {
		// 解析日志级别
		var loggerLevel logger.LogLevel
		switch strings.ToLower(logLevel) {
		case "silent":
			loggerLevel = logger.Silent
		case "error":
			loggerLevel = logger.Error
		case "warn":
			loggerLevel = logger.Warn
		case "info":
			loggerLevel = logger.Info
		default:
			loggerLevel = logger.Error
		}

		// 使用框架日志器适配器而不是标准输出
		gormLogger = logger.New(
			&GormLoggerAdapter{logger: appCtx.GetLogger()}, // 使用框架日志器适配
			logger.Config{
				SlowThreshold:             slowThreshold,
				LogLevel:                  loggerLevel,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      skipDefaultFields, // 对应 skipDefaultFields
				Colorful:                  colorful,
			},
		)
	} else {
		gormLogger = logger.Default.LogMode(logger.Silent)
	}

	// 创建数据库连接
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		Logger:                 gormLogger,
		// 根据需要添加其他配置
		// NamingStrategy: schema.NamingStrategy{
		//     TablePrefix: aConf.String(basePath + ".tablePrefix"),
		// },
	})

	if err != nil {
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginMysql()).Err(err).Msg("gorm.Open error")
		return nil, err
	}

	// 配置连接池
	sqlDb, err := db.DB()
	if err != nil {
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginMysql()).Err(err).Msg("db.DB() error")
		return nil, err
	}

	// 设置连接池参数
	sqlDb.SetMaxOpenConns(maxOpenConns)
	sqlDb.SetMaxIdleConns(maxIdleConns)
	sqlDb.SetConnMaxLifetime(connMaxLifetime)
	sqlDb.SetConnMaxIdleTime(connMaxIdleTime)

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDb.PingContext(ctx); err != nil {
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginMysql()).Err(err).Msg("mysql ping failed")
		return nil, err
	}

	return db, nil
}

func (md *MysqlDb) GetConfPath() string {
	return md.confPathname
}

// Close 关闭数据库连接
// 谨慎使用Close关闭链接
func (md *MysqlDb) Close() error {
	sqlDb, err := md.Client.DB()
	if err != nil {
		return err
	}
	return sqlDb.Close()
}

// IsHealthy 检查数据库连接是否健康
func (md *MysqlDb) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return md.PingTry(ctx)
}

// Rebuild 重新构建数据库连接，可以选择传入新的配置路径
func (md *MysqlDb) Rebuild(name ...interface{}) (interface{}, error) {
	if len(name) > 0 {
		return md.ReNewClient(name[0].(string))
	}
	return md.ReNewClient()
}

// ReNewClient 重新创建并替换当前的数据库客户端连接
func (md *MysqlDb) ReNewClient(confPath ...string) (*MysqlDb, error) {
	md.lock.Lock()
	defer md.lock.Unlock()

	client, errNc := NewClient(md.Ctx, confPath...)
	if errNc != nil {
		md.Ctx.GetLogger().Error(md.Ctx.GetConfig().LogOriginMysql()).Err(errNc).Msg("Mysql ReNewClient error")
		return md, errNc
	}

	md.Client = client
	return md, nil
}

// PingTry 尝试 ping 数据库以检查连接是否可用
func (md *MysqlDb) PingTry(ctx context.Context) bool {
	if md.Client == nil {
		md.Ctx.GetLogger().Error(md.Ctx.GetConfig().LogOriginMysql()).Msg("Mysql Client is nil, please check if the database connection is established")
		return false
	}
	sqlDb, err := md.Client.DB()
	if err != nil {
		md.Ctx.GetLogger().Error(md.Ctx.GetConfig().LogOriginMysql()).Err(err).Msg("Get sqlDb error")
		return false
	}
	if err := sqlDb.PingContext(ctx); err != nil {
		md.Ctx.GetLogger().Error(md.Ctx.GetConfig().LogOriginMysql()).Err(err).Msg("mysql ping failed")
		return false
	}
	md.Ctx.GetLogger().Info(md.Ctx.GetConfig().LogOriginMysql()).Msg("mysql ping successful")
	return true
}
