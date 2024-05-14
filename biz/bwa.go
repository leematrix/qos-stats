package biz

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"qos-stats/conf"
	"sync"
	"time"
)

type BwaStats struct {
	StreamID       float64 `json:"streamId"`
	MediaStatsBps  float64 `json:"mediaStatsBps"`
	ResendStatsBps float64 `json:"resendStatsBps"`
	FecStatsBps    float64 `json:"fecStatsBps"`
	MediaAllocBps  float64 `json:"mediaAllocBps"`
	ResendAllocBps float64 `json:"resendAllocBps"`
	FecAllocBps    float64 `json:"fecAllocBps"`
	EstimatedBps   float64 `json:"estimatedBps"`
	GopBps         float64 `json:"gopBps"`
	CreateTime     int64
}

type BwaStatsSession struct {
	BwaStatsChan  chan []byte
	BwaStatsQueue []BwaStats
	BwaStatsMutex sync.RWMutex
}

func BwaStatsSessionCreate() *BwaStatsSession {
	session := &BwaStatsSession{
		BwaStatsChan:  make(chan []byte, 1024),
		BwaStatsQueue: make([]BwaStats, 0),
	}
	return session
}

func (bwa *BwaStatsSession) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("bwa exit.")
			return
		case msg := <-bwa.BwaStatsChan:
			stats := BwaStats{
				CreateTime: time.Now().Unix(),
			}

			index := 0
			stats.StreamID = float64(msg[index])
			index += 1

			stats.MediaStatsBps = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.ResendStatsBps = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.FecStatsBps = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.MediaAllocBps = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.ResendAllocBps = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.FecAllocBps = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.EstimatedBps = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.GopBps = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			bwa.BwaStatsMutex.Lock()
			bwa.BwaStatsQueue = append(bwa.BwaStatsQueue, stats)
			if len(bwa.BwaStatsQueue) > conf.StatsWindowsCount {
				start := len(bwa.BwaStatsQueue) - conf.StatsWindowsCount
				bwa.BwaStatsQueue = bwa.BwaStatsQueue[start:]
			}
			bwa.BwaStatsMutex.Unlock()
		}
	}
}

func (bwa *BwaStatsSession) Incoming(msg []byte) {
	select {
	case bwa.BwaStatsChan <- msg:
	default:
	}
}

func (bwa *BwaStatsSession) Draw() ([]byte, error) {
	data := statsData{
		Legend:     []string{"MediaStats", "ResendStats", "FecStats", "MediaAlloc", "ResendAlloc", "FecAlloc", "Estimated", "Gop"},
		Series:     [][]float64{{}, {}, {}, {}, {}, {}, {}, {}},
		SeriesType: []string{"line", "line", "line", "line", "line", "line", "line", "line"},
	}
	bwa.BwaStatsMutex.RLock()
	for _, stats := range bwa.BwaStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.MediaStatsBps/1000)
		data.Series[1] = append(data.Series[1], stats.ResendStatsBps/1000)
		data.Series[2] = append(data.Series[2], stats.FecStatsBps/1000)
		data.Series[3] = append(data.Series[3], stats.MediaAllocBps/1000)
		data.Series[4] = append(data.Series[4], stats.ResendAllocBps/1000)
		data.Series[5] = append(data.Series[5], stats.FecAllocBps/1000)
		data.Series[6] = append(data.Series[6], stats.EstimatedBps/1000)
		data.Series[7] = append(data.Series[7], stats.GopBps/1000)
	}
	bwa.BwaStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (bwa *BwaStatsSession) Reset() {
	bwa.BwaStatsMutex.Lock()
	defer bwa.BwaStatsMutex.Unlock()
	bwa.BwaStatsQueue = bwa.BwaStatsQueue[:0]
	for len(bwa.BwaStatsChan) > 0 {
		select {
		case <-bwa.BwaStatsChan:
		default:
		}
	}
}
