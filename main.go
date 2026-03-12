package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xray-stat/internal/config"
	"xray-stat/internal/logs"
	"xray-stat/internal/ui"
	"xray-stat/internal/xrayapi"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "参数错误: %v\n", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	monitor := xrayapi.New(cfg.Addr, cfg.Tag)
	defer func() {
		_ = monitor.Close()
	}()

	logFollower, err := logs.NewFollower(cfg.LogMode, cfg.ErrorLog, cfg.AccessLog, cfg.LogLines)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "日志初始化警告: %v\n", err)
	}

	dashboard := ui.NewDashboard(cfg.Addr, cfg.Tag, cfg.LogMode, cfg.LogLines)
	logCh := make(chan logs.Entry, 512)
	if logFollower != nil {
		go logFollower.Run(ctx, logCh)
	}

	// 首屏立即渲染一次。
	snapshot := monitor.Snapshot(ctx)
	drainLogs(logCh, dashboard)
	dashboard.Render(snapshot)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_, _ = fmt.Fprint(os.Stdout, "\033[0m\n")
			return
		case entry := <-logCh:
			dashboard.PushLog(entry)
		case <-ticker.C:
			snapshot = monitor.Snapshot(ctx)
			drainLogs(logCh, dashboard)
			dashboard.Render(snapshot)
		}
	}
}

func drainLogs(logCh <-chan logs.Entry, d *ui.Dashboard) {
	for {
		select {
		case item := <-logCh:
			d.PushLog(item)
		default:
			return
		}
	}
}
