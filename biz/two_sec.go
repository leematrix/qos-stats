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

type SenderTwoSecStats struct {
	StreamID    float64
	FrameRate   float64
	CollectTime float64
	CreateTime  int64
}

type ReceiverTwoSecStats struct {
	StreamID         float64
	FrameRate        float64
	MaxRetryCount    float64
	MaxRetryDuration float64
	CollectTime      float64
	CreateTime       int64
}

type TwoSecStatsSession struct {
	SenderTwoSecStatsChan  chan []byte
	SenderTwoSecStatsQueue []SenderTwoSecStats
	SenderTwoSecStatsMutex sync.RWMutex

	ReceiverTwoSecStatsChan  chan []byte
	ReceiverTwoSecStatsQueue []ReceiverTwoSecStats
	ReceiverTwoSecStatsMutex sync.RWMutex
}

func TwoStatsSessionCreate() *TwoSecStatsSession {
	session := &TwoSecStatsSession{
		SenderTwoSecStatsChan:    make(chan []byte, 1024),
		SenderTwoSecStatsQueue:   make([]SenderTwoSecStats, 0),
		ReceiverTwoSecStatsChan:  make(chan []byte, 1024),
		ReceiverTwoSecStatsQueue: make([]ReceiverTwoSecStats, 0),
	}
	return session
}

func (tweSec *TwoSecStatsSession) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("tweSec exit.")
			return
		case msg := <-tweSec.SenderTwoSecStatsChan:
			stats := SenderTwoSecStats{
				CreateTime: time.Now().Unix(),
			}

			index := 0
			stats.StreamID = float64(msg[index])
			index += 1

			stats.FrameRate = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.CollectTime = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			tweSec.SenderTwoSecStatsMutex.Lock()
			tweSec.SenderTwoSecStatsQueue = append(tweSec.SenderTwoSecStatsQueue, stats)
			if len(tweSec.SenderTwoSecStatsQueue) > conf.StatsWindowsCount {
				start := len(tweSec.SenderTwoSecStatsQueue) - conf.StatsWindowsCount
				tweSec.SenderTwoSecStatsQueue = tweSec.SenderTwoSecStatsQueue[start:]
			}
			tweSec.SenderTwoSecStatsMutex.Unlock()
		case msg := <-tweSec.ReceiverTwoSecStatsChan:
			stats := ReceiverTwoSecStats{
				CreateTime: time.Now().Unix(),
			}

			index := 0
			stats.StreamID = float64(msg[index])
			index += 1

			stats.FrameRate = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.MaxRetryCount = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.MaxRetryDuration = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.CollectTime = float64(binary.BigEndian.Uint64(msg[index:]))
			index += 8

			tweSec.ReceiverTwoSecStatsMutex.Lock()
			tweSec.ReceiverTwoSecStatsQueue = append(tweSec.ReceiverTwoSecStatsQueue, stats)
			if len(tweSec.ReceiverTwoSecStatsQueue) > conf.StatsWindowsCount {
				start := len(tweSec.ReceiverTwoSecStatsQueue) - conf.StatsWindowsCount
				tweSec.ReceiverTwoSecStatsQueue = tweSec.ReceiverTwoSecStatsQueue[start:]
			}
			tweSec.ReceiverTwoSecStatsMutex.Unlock()
		}
	}
}

func (tweSec *TwoSecStatsSession) SenderIncoming(msg []byte) {
	select {
	case tweSec.SenderTwoSecStatsChan <- msg:
	default:
	}
}

func (tweSec *TwoSecStatsSession) ReceiverIncoming(msg []byte) {
	select {
	case tweSec.ReceiverTwoSecStatsChan <- msg:
	default:
	}
}

func (tweSec *TwoSecStatsSession) SenderFrameRateDraw() ([]byte, error) {
	data := statsData{
		Series: [][]float64{{}, {}, {}, {}, {}},
	}
	tweSec.SenderTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.SenderTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.FrameRate)
	}
	tweSec.SenderTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) ReceiverFrameRateDraw() ([]byte, error) {
	data := statsData{
		Series: [][]float64{{}, {}, {}, {}, {}},
	}
	tweSec.ReceiverTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.ReceiverTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.FrameRate)
	}
	tweSec.ReceiverTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) NackCountDraw() ([]byte, error) {
	data := statsData{
		Series: [][]float64{{}, {}, {}, {}, {}},
	}
	tweSec.ReceiverTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.ReceiverTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.MaxRetryCount)
	}
	tweSec.ReceiverTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) NackDurationDraw() ([]byte, error) {
	data := statsData{
		Series: [][]float64{{}, {}, {}, {}, {}},
	}
	tweSec.ReceiverTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.ReceiverTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.MaxRetryDuration)
	}
	tweSec.ReceiverTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) Reset() {
	tweSec.ReceiverTwoSecStatsMutex.Lock()
	defer tweSec.ReceiverTwoSecStatsMutex.Unlock()
	tweSec.ReceiverTwoSecStatsQueue = tweSec.ReceiverTwoSecStatsQueue[:0]
	for len(tweSec.ReceiverTwoSecStatsChan) > 0 {
		select {
		case <-tweSec.ReceiverTwoSecStatsChan:
		default:
		}
	}
}
