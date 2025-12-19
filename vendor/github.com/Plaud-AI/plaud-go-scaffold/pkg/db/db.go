package db

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/svc"

	gormlogger "gorm.io/gorm/logger"

	"github.com/Plaud-AI/plaud-library-go/env"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Client 基于 GORM 的数据库客户端（MySQL / Postgres）
type Client struct {
	svc.BaseService
	db *gorm.DB
}

func (s *Client) Stop(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// NewClient 基于 config.DBConfig 初始化 GORM 客户端（依据 cfg.Type 选择驱动）
// PoolSize 控制连接池最大连接数，空则默认 20
func NewClient(cfg *config.DBConfig) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("db config is nil")
	}
	dsn := strings.TrimSpace(cfg.DSN)
	if dsn == "" {
		return nil, errors.New("db dsn is empty")
	}

	// 依据 cfg.Type 选择驱动（cfg.Type 为必填，已在 Parse 校验）
	dbType := strings.ToLower(strings.TrimSpace(cfg.Type))

	var dialector gorm.Dialector
	switch dbType {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		fallthrough
	default:
		dialector = mysql.Open(dsn)
	}

	logMode := gormlogger.Silent
	switch env.GetEnv() {
	case env.DevelopEnv, env.TestEnv:
		logMode = gormlogger.Info
	}

	gormLog := gormlogger.New(
		log.New(logger.GetLoggerWriter(), "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logMode,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormLog,
	})

	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	maxOpen := cfg.PoolSize
	if maxOpen <= 0 {
		maxOpen = 20
	}
	maxIdle := maxOpen / 2
	if maxIdle < 2 {
		maxIdle = 2
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	logger.Infof("connected to db: %s", redactDSN(dsn))
	return &Client{db: db}, nil
}

// GetDB 获取底层 *gorm.DB
func (p *Client) GetDB() *gorm.DB {
	return p.db
}

// Close 关闭底层连接池
func (p *Client) Close() error {
	if p.IsStopped() {
		return nil
	}
	if p == nil || p.db == nil {
		return nil
	}
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	p.SetStopped(true)
	return sqlDB.Close()
}

// redactDSN 屏蔽 DSN 中的密码，仅用于日志
func redactDSN(dsn string) string {
	at := strings.Index(dsn, "@")
	colon := strings.Index(dsn, ":")
	if colon > 0 && at > 0 && colon < at {
		return dsn[:colon+1] + "****" + dsn[at:]
	}
	return dsn
}
