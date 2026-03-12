package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LogModeErrorOnly  = "error-only"
	LogModeMerge      = "merge"
	LogModeAccessOnly = "access-only"
)

var validLogModes = map[string]struct{}{
	LogModeErrorOnly:  {},
	LogModeMerge:      {},
	LogModeAccessOnly: {},
}

type Config struct {
	Addr      string
	Tag       string
	Interval  time.Duration
	LogLines  int
	ErrorLog  string
	AccessLog string
	LogMode   string
}

func Parse() (Config, error) {
	var cfg Config
	var intervalSeconds int

	flag.StringVar(&cfg.Addr, "addr", "127.0.0.1:10085", "Xray API gRPC 地址")
	flag.StringVar(&cfg.Tag, "tag", "proxy", "要监控的 outbound tag")
	flag.IntVar(&intervalSeconds, "i", 1, "刷新间隔（秒）")
	flag.IntVar(&cfg.LogLines, "loglines", 10, "日志显示行数")
	flag.StringVar(&cfg.ErrorLog, "error-log", "~/.xray/error.log", "error 日志文件路径")
	flag.StringVar(&cfg.AccessLog, "access-log", "~/.xray/access.log", "access 日志文件路径")
	flag.StringVar(&cfg.LogMode, "logmode", LogModeErrorOnly, "日志显示模式：error-only / merge / access-only")
	flag.Parse()

	if cfg.Addr == "" {
		return cfg, errors.New("-addr 不能为空")
	}
	if cfg.Tag == "" {
		return cfg, errors.New("-tag 不能为空")
	}
	if intervalSeconds <= 0 {
		return cfg, errors.New("-i 必须大于 0")
	}
	if cfg.LogLines <= 0 {
		return cfg, errors.New("-loglines 必须大于 0")
	}
	if _, ok := validLogModes[cfg.LogMode]; !ok {
		return cfg, fmt.Errorf("-logmode 无效：%s（可选 error-only / merge / access-only）", cfg.LogMode)
	}

	var err error
	cfg.ErrorLog, err = expandPath(cfg.ErrorLog)
	if err != nil {
		return cfg, fmt.Errorf("展开 error-log 失败: %w", err)
	}
	cfg.AccessLog, err = expandPath(cfg.AccessLog)
	if err != nil {
		return cfg, fmt.Errorf("展开 access-log 失败: %w", err)
	}

	cfg.Interval = time.Duration(intervalSeconds) * time.Second
	return cfg, nil
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Clean(path), nil
}
