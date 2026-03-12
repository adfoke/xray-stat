package xrayapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	observatorycmd "github.com/xtls/xray-core/app/observatory/command"
	statscmd "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	dialTimeout = 2 * time.Second
	rpcTimeout  = 2 * time.Second
)

type Snapshot struct {
	Timestamp time.Time
	Connected bool
	LatencyMS int64
	UpRate    float64
	DownRate  float64
	TotalUp   uint64
	TotalDown uint64
	APIError  error
}

type Client struct {
	addr string
	tag  string

	mu       sync.Mutex
	conn     *grpc.ClientConn
	stats    statscmd.StatsServiceClient
	obs      observatorycmd.ObservatoryServiceClient
	lastAt   time.Time
	lastUp   uint64
	lastDown uint64
}

func New(addr, tag string) *Client {
	return &Client{
		addr: addr,
		tag:  tag,
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.stats = nil
	c.obs = nil
	return err
}

func (c *Client) Snapshot(ctx context.Context) Snapshot {
	now := time.Now()
	out := Snapshot{
		Timestamp: now,
		Connected: false,
		LatencyMS: -1,
	}

	if err := c.ensureConn(ctx); err != nil {
		out.APIError = err
		return out
	}

	up, upErr := c.getTraffic(ctx, "uplink")
	down, downErr := c.getTraffic(ctx, "downlink")
	if upErr != nil || downErr != nil {
		out.APIError = joinErr(upErr, downErr)
		c.invalidateConn()
		return out
	}
	out.TotalUp = up
	out.TotalDown = down

	connected, latency, obsErr := c.getOutboundStatus(ctx)
	if obsErr != nil {
		out.APIError = obsErr
		connected = true
		latency = -1
	}
	out.Connected = connected
	out.LatencyMS = latency

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.lastAt.IsZero() {
		elapsed := now.Sub(c.lastAt).Seconds()
		if elapsed > 0 {
			if up >= c.lastUp {
				out.UpRate = float64(up-c.lastUp) / elapsed
			}
			if down >= c.lastDown {
				out.DownRate = float64(down-c.lastDown) / elapsed
			}
		}
	}
	c.lastAt = now
	c.lastUp = up
	c.lastDown = down
	return out
}

func (c *Client) ensureConn(ctx context.Context) error {
	c.mu.Lock()
	if c.conn != nil && c.stats != nil && c.obs != nil {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		c.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("连接 Xray API 失败 (%s): %w", c.addr, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = conn.Close()
		return nil
	}
	c.conn = conn
	c.stats = statscmd.NewStatsServiceClient(conn)
	c.obs = observatorycmd.NewObservatoryServiceClient(conn)
	return nil
}

func (c *Client) invalidateConn() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.stats = nil
	c.obs = nil
}

func (c *Client) getTraffic(ctx context.Context, direction string) (uint64, error) {
	name := fmt.Sprintf("outbound>>>%s>>>traffic>>>%s", c.tag, direction)
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	c.mu.Lock()
	client := c.stats
	c.mu.Unlock()
	if client == nil {
		return 0, errors.New("StatsService 未初始化")
	}

	resp, err := client.GetStats(rpcCtx, &statscmd.GetStatsRequest{
		Name:   name,
		Reset_: false,
	})
	if err != nil {
		return 0, fmt.Errorf("读取流量失败 [%s]: %w", name, err)
	}
	if resp.GetStat() == nil {
		return 0, fmt.Errorf("流量统计项不存在 [%s]", name)
	}
	val := resp.GetStat().GetValue()
	if val < 0 {
		return 0, nil
	}
	return uint64(val), nil
}

func (c *Client) getOutboundStatus(ctx context.Context) (connected bool, latencyMS int64, err error) {
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	c.mu.Lock()
	client := c.obs
	c.mu.Unlock()
	if client == nil {
		return false, -1, errors.New("ObservatoryService 未初始化")
	}

	resp, err := client.GetOutboundStatus(rpcCtx, &observatorycmd.GetOutboundStatusRequest{})
	if err != nil {
		return false, -1, fmt.Errorf("读取延迟状态失败: %w", err)
	}
	result := resp.GetStatus()
	if result == nil || len(result.GetStatus()) == 0 {
		return false, -1, errors.New("Observatory 返回空状态")
	}

	for _, item := range result.GetStatus() {
		if strings.EqualFold(item.GetOutboundTag(), c.tag) {
			return item.GetAlive(), item.GetDelay(), nil
		}
	}

	// tag 未命中时，退化使用第一项，避免界面完全空白。
	first := result.GetStatus()[0]
	return first.GetAlive(), first.GetDelay(), fmt.Errorf("未找到 tag=%s，已使用 %s", c.tag, first.GetOutboundTag())
}

func joinErr(errs ...error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}
