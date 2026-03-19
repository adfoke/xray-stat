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
	scrollOffset  int
}

type logRange struct {
	start int
	end   int
	total int
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
	if d.scrollOffset > 0 {
		d.scrollOffset++
	}
	if len(d.logBuffer) > d.maxStoredLogs {
		d.logBuffer = d.logBuffer[len(d.logBuffer)-d.maxStoredLogs:]
	}
	d.clampScrollOffset(d.visibleLogLines())
}

func (d *Dashboard) Render(snapshot xrayapi.Snapshot) {
	logLines := d.visibleLogLines()
	displayLogs := d.visibleLogs(logLines)
	logWidth := d.visibleLogWidth()
	logRange := d.visibleLogRange(logLines)

	var b strings.Builder
	b.WriteString(clear)

	b.WriteString(fmt.Sprintf("%s%sXray Terminal Monitor (xray-stat)%s\n", bold, cyan, reset))
	b.WriteString(fmt.Sprintf("%sTag:%s %s   %sAPI:%s %s   %s刷新:%s %s\n",
		bold, reset, d.tag, bold, reset, d.addr, bold, reset, snapshot.Timestamp.Format("2006-01-02 15:04:05")))

	statusText := "DOWN"
	statusColor := red
	if snapshot.Connected {
		statusText = "OK"
		statusColor = green
	}
	latencyText := "N/A"
	if snapshot.LatencyMS >= 0 {
		latencyText = fmt.Sprintf("%d ms", snapshot.LatencyMS)
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s状态%s 连接: %s%s%s   延迟: %s%s%s\n",
		bold, reset, statusColor, statusText, reset, cyan, latencyText, reset))

	b.WriteString(fmt.Sprintf("%s流量%s 实时 UP %s/s   DOWN %s/s\n",
		bold, reset, formatBytes(snapshot.UpRate), formatBytes(snapshot.DownRate)))
	b.WriteString(fmt.Sprintf("     累计 UP %s   DOWN %s\n",
		formatBytes(float64(snapshot.TotalUp)), formatBytes(float64(snapshot.TotalDown))))

	if snapshot.APIError != nil {
		b.WriteString(fmt.Sprintf("%sAPI:%s %s%s%s\n", bold, reset, yellow, snapshot.APIError.Error(), reset))
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s日志%s 模式=%s   顺序=最新在上   滚动=j/k 或 ↑/↓   翻页=PgUp/PgDn   置顶/底=g/G   Ctrl+C 退出\n",
		bold, reset, d.logMode))
	if logRange.total > 0 {
		b.WriteString(fmt.Sprintf("显示 %d-%d / %d\n", logRange.start, logRange.end, logRange.total))
	}
	b.WriteString(renderLogBox(displayLogs, logLines, logWidth))

	_, _ = os.Stdout.WriteString(b.String())
}

func (d *Dashboard) ScrollOlder(step int) {
	if step <= 0 {
		return
	}
	d.scrollOffset += step
	d.clampScrollOffset(d.visibleLogLines())
}

func (d *Dashboard) ScrollNewer(step int) {
	if step <= 0 {
		return
	}
	d.scrollOffset -= step
	if d.scrollOffset < 0 {
		d.scrollOffset = 0
	}
}

func (d *Dashboard) ScrollToNewest() {
	d.scrollOffset = 0
}

func (d *Dashboard) ScrollToOldest() {
	d.scrollOffset = d.maxScrollOffset(d.visibleLogLines())
}

func (d *Dashboard) PageSize() int {
	size := d.visibleLogLines()
	if size <= 0 {
		return d.baseLogLines
	}
	return size
}

func (d *Dashboard) visibleLogs(n int) []logs.Entry {
	if n <= 0 {
		return nil
	}
	d.clampScrollOffset(n)
	if len(d.logBuffer) == 0 {
		return nil
	}
	start := len(d.logBuffer) - 1 - d.scrollOffset
	if start < 0 {
		return nil
	}
	end := start - n + 1
	if end < 0 {
		end = 0
	}
	result := make([]logs.Entry, 0, start-end+1)
	for i := start; i >= end; i-- {
		result = append(result, d.logBuffer[i])
	}
	return result
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
	dynamic := total - 14
	if dynamic < d.baseLogLines {
		return d.baseLogLines
	}
	return dynamic
}

func (d *Dashboard) visibleLogWidth() int {
	colsEnv := os.Getenv("COLUMNS")
	if colsEnv == "" {
		return 100
	}
	total, err := strconv.Atoi(colsEnv)
	if err != nil || total <= 0 {
		return 100
	}
	width := total - 6
	switch {
	case width < 40:
		return 40
	case width > 140:
		return 140
	default:
		return width
	}
}

func (d *Dashboard) clampScrollOffset(logLines int) {
	maxOffset := d.maxScrollOffset(logLines)
	if d.scrollOffset > maxOffset {
		d.scrollOffset = maxOffset
	}
	if d.scrollOffset < 0 {
		d.scrollOffset = 0
	}
}

func (d *Dashboard) maxScrollOffset(logLines int) int {
	if logLines <= 0 {
		return 0
	}
	total := len(d.logBuffer)
	if total <= logLines {
		return 0
	}
	return total - logLines
}

func (d *Dashboard) visibleLogRange(logLines int) logRange {
	total := len(d.logBuffer)
	if total == 0 {
		return logRange{}
	}
	d.clampScrollOffset(logLines)
	start := d.scrollOffset + 1
	end := d.scrollOffset + logLines
	if end > total {
		end = total
	}
	return logRange{
		start: start,
		end:   end,
		total: total,
	}
}

func renderLogBox(entries []logs.Entry, height, width int) string {
	if height <= 0 {
		return ""
	}
	var b strings.Builder
	border := "+" + strings.Repeat("-", width+2) + "+\n"
	b.WriteString(border)
	if len(entries) == 0 {
		b.WriteString(renderLogRow("暂无日志输出，等待新日志...", width, gray))
		for i := 1; i < height; i++ {
			b.WriteString(renderLogRow("", width, ""))
		}
		b.WriteString(border)
		return b.String()
	}
	for i := 0; i < height; i++ {
		if i < len(entries) {
			line := fmt.Sprintf("[%s] %s", entries[i].Source, entries[i].Line)
			b.WriteString(renderLogRow(line, width, logColor(line)))
			continue
		}
		b.WriteString(renderLogRow("", width, ""))
	}
	b.WriteString(border)
	return b.String()
}

func renderLogRow(line string, width int, color string) string {
	text := fitText(sanitizeLogLine(line), width)
	if color != "" {
		text = color + text + reset
	}
	return "| " + text + " |\n"
}

func sanitizeLogLine(line string) string {
	line = strings.ReplaceAll(line, "\n", " ")
	line = strings.ReplaceAll(line, "\t", "    ")
	return line
}

func fitText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) > width {
		if width <= 3 {
			return string(runes[:width])
		}
		return string(runes[:width-3]) + "..."
	}
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
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
	color := logColor(line)
	if color == "" {
		return line
	}
	return color + line + reset
}

func logColor(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error"), strings.Contains(lower, "failed"),
		strings.Contains(lower, "panic"), strings.Contains(lower, "fatal"):
		return red
	case strings.Contains(lower, "warning"):
		return yellow
	case strings.Contains(lower, "info"), strings.Contains(lower, "debug"):
		return green
	default:
		return ""
	}
}
