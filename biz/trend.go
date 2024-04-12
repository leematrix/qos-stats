package biz

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"math"
	"qos-stats/conf"
	"sync"
	"time"
)

type TrendStats struct {
	ModifiedTrend float64
	Threshold     float64
	CreateTime    int64
}

type TrendStatsSession struct {
	TrendStatsChan  chan []byte
	TrendStatsQueue []TrendStats
	TrendStatsMutex sync.RWMutex
}

func TrendStatsSessionCreate() *TrendStatsSession {
	session := &TrendStatsSession{
		TrendStatsChan:  make(chan []byte, 1024),
		TrendStatsQueue: make([]TrendStats, 0),
	}
	return session
}

func (trend *TrendStatsSession) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("trend exit.")
			return
		case msg := <-trend.TrendStatsChan:
			stats := TrendStats{
				CreateTime: time.Now().Unix(),
			}

			index := 0
			stats.ModifiedTrend = math.Float64frombits(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			stats.Threshold = math.Float64frombits(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			trend.TrendStatsMutex.Lock()
			trend.TrendStatsQueue = append(trend.TrendStatsQueue, stats)
			if len(trend.TrendStatsQueue) > conf.StatsWindowsCount {
				start := len(trend.TrendStatsQueue) - conf.StatsWindowsCount
				trend.TrendStatsQueue = trend.TrendStatsQueue[start:]
			}
			trend.TrendStatsMutex.Unlock()
		}
	}
}

func (trend *TrendStatsSession) Incoming(msg []byte) {
	select {
	case trend.TrendStatsChan <- msg:
	default:
	}
}

func (trend *TrendStatsSession) Draw() ([]byte, error) {
	data := statsData{
		Series: [][]float64{{}, {}, {}},
	}
	trend.TrendStatsMutex.RLock()
	for _, stats := range trend.TrendStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.Threshold)
		data.Series[1] = append(data.Series[1], stats.ModifiedTrend)
		data.Series[2] = append(data.Series[2], -stats.Threshold)
	}
	trend.TrendStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (trend *TrendStatsSession) Reset() {
	trend.TrendStatsMutex.Lock()
	defer trend.TrendStatsMutex.Unlock()
	trend.TrendStatsQueue = trend.TrendStatsQueue[:0]
	for len(trend.TrendStatsChan) > 0 {
		select {
		case <-trend.TrendStatsChan:
		default:
		}
	}
}
