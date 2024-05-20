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
	StreamID        float64
	FrameRate       float64
	CollectTime     float64
	MediaCount      float64
	MediaSize       float64
	RetransmitCount float64
	RetransmitSize  float64
	FecCount        float64
	FecSize         float64
	CreateTime      int64
}

type ReceiverTwoSecStats struct {
	StreamID         float64
	FrameRate        float64
	MaxRetryCount    float64
	MaxRetryDuration float64
	CollectTime      float64
	MediaCount       float64
	RetransmitCount  float64
	FecRecoveryCount float64
	StutterCount     float64
	DropFrameCount   float64 // 丢帧次数
	StutterDuration  float64 // 最大卡顿时长
	MaxBufferLength  float64 // 最大缓冲长度
	MinBufferLength  float64 // 最小缓冲长度
	AvgBufferLength  float64 // 平均缓冲长度
	MaxJitterDelay   float64 // 最大抖动延迟
	MinJitterDelay   float64 // 最小抖动延迟
	MediaTimeMs      float64 // 媒体时间戳
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

			stats.MediaSize = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.MediaCount = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.RetransmitSize = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.RetransmitCount = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.FecSize = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.FecCount = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

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

			stats.MediaCount = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.RetransmitCount = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.FecRecoveryCount = float64(binary.BigEndian.Uint32(msg[index:]))
			index += 4

			stats.StutterCount = float64(msg[index])
			index += 1

			stats.DropFrameCount = float64(msg[index])
			index += 1

			stats.StutterDuration = float64(binary.BigEndian.Uint16(msg[index:]))
			index += 2

			stats.MaxBufferLength = float64(binary.BigEndian.Uint16(msg[index:]))
			index += 2

			stats.MinBufferLength = float64(binary.BigEndian.Uint16(msg[index:]))
			index += 2

			stats.AvgBufferLength = float64(binary.BigEndian.Uint16(msg[index:]))
			index += 2

			stats.MaxJitterDelay = float64(binary.BigEndian.Uint16(msg[index:]))
			index += 2

			stats.MinJitterDelay = float64(binary.BigEndian.Uint16(msg[index:]))
			index += 2

			stats.MediaTimeMs = float64(binary.BigEndian.Uint64(msg[index:]))
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
		Legend:     []string{""},
		Series:     [][]float64{{}},
		SeriesType: []string{"line"},
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
		Legend:     []string{""},
		Series:     [][]float64{{}},
		SeriesType: []string{"line"},
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
		Legend:     []string{""},
		Series:     [][]float64{{}},
		SeriesType: []string{"line"},
	}
	tweSec.ReceiverTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.ReceiverTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.MaxRetryCount)
	}
	tweSec.ReceiverTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) NackCostDraw() ([]byte, error) {
	data := statsData{
		Legend:     []string{""},
		Series:     [][]float64{{}},
		SeriesType: []string{"line"},
	}
	tweSec.ReceiverTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.ReceiverTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.MaxRetryDuration)
	}
	tweSec.ReceiverTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) SendRateDraw() ([]byte, error) {
	data := statsData{
		Legend:     []string{"Media", "Retransmit", "Fec"},
		Series:     [][]float64{{}, {}, {}},
		SeriesType: []string{"line", "line", "line"},
	}
	tweSec.SenderTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.SenderTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.MediaSize/250)
		data.Series[1] = append(data.Series[1], stats.RetransmitSize/250)
		data.Series[2] = append(data.Series[2], stats.FecSize/250)
	}
	tweSec.SenderTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) SendCountDraw() ([]byte, error) {
	data := statsData{
		Legend:     []string{"Media", "Retransmit", "Fec"},
		Series:     [][]float64{{}, {}, {}},
		SeriesType: []string{"line", "line", "line"},
	}
	tweSec.SenderTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.SenderTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.MediaCount)
		data.Series[1] = append(data.Series[1], stats.RetransmitCount)
		data.Series[2] = append(data.Series[2], stats.FecCount)
	}
	tweSec.SenderTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) RecvCountDraw() ([]byte, error) {
	data := statsData{
		Legend:     []string{"Media", "Retransmit", "Fec"},
		Series:     [][]float64{{}, {}, {}},
		SeriesType: []string{"line", "line", "line"},
	}
	tweSec.ReceiverTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.ReceiverTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.MediaCount)
		data.Series[1] = append(data.Series[1], stats.RetransmitCount)
		data.Series[2] = append(data.Series[2], stats.FecRecoveryCount)
	}
	tweSec.ReceiverTwoSecStatsMutex.RUnlock()
	return json.Marshal(data)
}

func (tweSec *TwoSecStatsSession) JitterDraw() ([]byte, error) {
	data := statsData{
		Legend:     []string{"StutterCount", "StutterDuration", "DropFrameCount", "MaxBufferLength", "MinBufferLength", "AvgBufferLength", "MaxJitterDelay", "MinJitterDelay", "MediaTimeMs"},
		Series:     [][]float64{{}, {}, {}, {}, {}, {}, {}, {}, {}},
		SeriesType: []string{"line", "line", "line", "line", "line", "line", "line", "line", "line"},
	}
	tweSec.ReceiverTwoSecStatsMutex.RLock()
	for _, stats := range tweSec.ReceiverTwoSecStatsQueue {
		data.XAxis = append(data.XAxis, time.Unix(stats.CreateTime, 0).Format("15:04:05"))
		data.Series[0] = append(data.Series[0], stats.StutterCount)
		data.Series[1] = append(data.Series[1], stats.StutterDuration)
		data.Series[2] = append(data.Series[2], stats.DropFrameCount)
		data.Series[3] = append(data.Series[3], stats.MaxBufferLength)
		data.Series[4] = append(data.Series[4], stats.MinBufferLength)
		data.Series[5] = append(data.Series[5], stats.AvgBufferLength)
		data.Series[6] = append(data.Series[6], stats.MaxJitterDelay)
		data.Series[7] = append(data.Series[7], stats.MinJitterDelay)
		data.Series[8] = append(data.Series[8], stats.MediaTimeMs)
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
