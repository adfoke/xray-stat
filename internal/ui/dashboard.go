package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"xray-stat/internal/logs"
	"xray-stat/internal/xrayapi"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
	clear  = "\033[2J\033[H"
)

type Dashboard struct {
	addr          string
	tag           string
	logMode       string
	baseLogLines  int
	logBuffer     []logs.Entry
	maxStoredLogs int
}

func NewDashboard(addr, tag, logMode string, logLines int) *Dashboard {
	maxStored := logLines * 80
	if maxStored < 800 {
		maxStored = 800
	}
	return &Dashboard{
		addr:          addr,
		tag:           tag,
		logMode:       logMode,
		baseLogLines:  logLines,
		maxStoredLogs: maxStored,
	}
}

func (d *Dashboard) PushLog(entry logs.Entry) {
	d.logBuffer = append(d.logBuffer, entry)
	if len(d.logBuffer) > d.maxStoredLogs {
		d.logBuffer = d.logBuffer[len(d.logBuffer)-d.maxStoredLogs:]
	}
}

func (d *Dashboard) Render(snapshot xrayapi.Snapshot) {
	logLines := d.visibleLogLines()
	displayLogs := d.lastNLogs(logLines)

	var b strings.Builder
	b.WriteString(clear)

	b.WriteString(fmt.Sprintf("%s%sXray Terminal Monitor (xray-stat)%s\n", bold, cyan, reset))
	b.WriteString(fmt.Sprintf("%sTag:%s %s   %sAPI:%s %s   %s刷新:%s %s\n",
		bold, reset, d.tag, bold, reset, d.addr, bold, reset, snapshot.Timestamp.Format("2006-01-02 15:04:05")))

	statusText := "❌ 断开"
	statusColor := red
	if snapshot.Connected {
		statusText = "✅ 正常"
		statusColor = green
	}
	latencyText := "N/A"
	if snapshot.LatencyMS >= 0 {
		latencyText = fmt.Sprintf("%d ms", snapshot.LatencyMS)
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s状态%s 连接: %s%s%s   延迟: %s%s%s\n",
		bold, reset, statusColor, statusText, reset, cyan, latencyText, reset))

	b.WriteString(fmt.Sprintf("%s流量%s 实时 ↑ %s/s   ↓ %s/s\n",
		bold, reset, formatBytes(snapshot.UpRate), formatBytes(snapshot.DownRate)))
	b.WriteString(fmt.Sprintf("     累计 ↑ %s   ↓ %s\n",
		formatBytes(float64(snapshot.TotalUp)), formatBytes(float64(snapshot.TotalDown))))

	if snapshot.APIError != nil {
		b.WriteString(fmt.Sprintf("%sAPI:%s %s%s%s\n", bold, reset, yellow, snapshot.APIError.Error(), reset))
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s日志%s 模式=%s (Ctrl+C 退出)\n", bold, reset, d.logMode))
	b.WriteString(strings.Repeat("─", 64))
	b.WriteString("\n")

	if len(displayLogs) == 0 {
		b.WriteString(gray)
		b.WriteString("暂无日志输出，等待新日志...\n")
		b.WriteString(reset)
	} else {
		for _, item := range displayLogs {
			b.WriteString(colorizeLog(fmt.Sprintf("[%s] %s", item.Source, item.Line)))
			b.WriteString("\n")
		}
	}

	_, _ = os.Stdout.WriteString(b.String())
}

func (d *Dashboard) lastNLogs(n int) []logs.Entry {
	if n <= 0 {
		return nil
	}
	if len(d.logBuffer) <= n {
		return d.logBuffer
	}
	return d.logBuffer[len(d.logBuffer)-n:]
}

func (d *Dashboard) visibleLogLines() int {
	linesEnv := os.Getenv("LINES")
	if linesEnv == "" {
		return d.baseLogLines
	}
	total, err := strconv.Atoi(linesEnv)
	if err != nil || total <= 0 {
		return d.baseLogLines
	}
	dynamic := int(float64(total)*0.65) - 8
	if dynamic < d.baseLogLines {
		return d.baseLogLines
	}
	return dynamic
}

func formatBytes(bytes float64) string {
	const (
		KB = 1024.0
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", bytes/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", bytes/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", bytes/KB)
	default:
		return fmt.Sprintf("%.0f B", bytes)
	}
}

func colorizeLog(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error"), strings.Contains(lower, "failed"),
		strings.Contains(lower, "panic"), strings.Contains(lower, "fatal"):
		return red + line + reset
	case strings.Contains(lower, "warning"):
		return yellow + line + reset
	case strings.Contains(lower, "info"), strings.Contains(lower, "debug"):
		return green + line + reset
	default:
		return line
	}
}
