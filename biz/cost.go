package biz

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"qos-stats/conf"
	"strconv"
	"sync"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

const (
	CostStatsMessageFrameTypeStart   = 0
	CostStatsMessageFrameTypeEnqueue = 1
	CostStatsMessageFrameTypeDequeue = 2
	CostStatsMessageFrameTypeSend    = 3
	CostStatsMessageFrameTypeRecv    = 4
	CostStatsMessageFrameTypeDrop    = 5
	CostStatsMessageFrameTypeEnd     = 6
)

type utpUnit struct {
	seq       int64
	isRetry   bool
	atTime    int64
	statsType uint8
}

type frameUnit struct {
	seq         int64
	startTime   int64
	enqueueTime int64
	dequeueTime int64
	sendTime    int64
	recvTime    int64
	dropTime    int64
	endTime     int64
	utpMap      map[int64]utpUnit
	CreateTime  int64
}

type frameStatsMessage struct {
	frameSeq  int64
	utpSeq    int64
	isRetry   bool
	atTime    int64
	isSend    bool
	statsType uint8
	streamId  uint8
}

type frameMap map[int64]*frameUnit

type streamMap struct {
	streamId    uint8
	frameMap    frameMap
	frameMinSeq int64
	frameMaxSeq int64
}

var streams = make(map[uint8]streamMap)
var frameStatsChan = make(chan frameStatsMessage, 10000)
var frameStatsMutex sync.RWMutex

func frameStatsSet(msg frameStatsMessage, unit *frameUnit) {
	switch msg.statsType {
	case CostStatsMessageFrameTypeStart:
		if unit.startTime == 0 {
			unit.startTime = msg.atTime
		}
	case CostStatsMessageFrameTypeEnqueue:
		if msg.atTime < unit.enqueueTime || unit.enqueueTime == 0 {
			unit.enqueueTime = msg.atTime
		}
	case CostStatsMessageFrameTypeDequeue:
		if msg.atTime > unit.dequeueTime {
			if (unit.endTime == 0 && unit.dropTime == 0) ||
				(msg.atTime < unit.endTime && unit.endTime != 0) ||
				(msg.atTime < unit.dropTime && unit.dropTime != 0) {
				unit.dequeueTime = msg.atTime
			}
		}
	case CostStatsMessageFrameTypeSend:
		if msg.atTime < unit.sendTime || unit.sendTime == 0 {
			unit.sendTime = msg.atTime
		}
	case CostStatsMessageFrameTypeRecv:
		if msg.atTime > unit.recvTime {
			if (unit.endTime == 0 && unit.dropTime == 0) ||
				(msg.atTime < unit.endTime && unit.endTime != 0) ||
				(msg.atTime < unit.dropTime && unit.dropTime != 0) {
				unit.recvTime = msg.atTime
			}
		}
	case CostStatsMessageFrameTypeDrop:
		if unit.dropTime == 0 {
			unit.dropTime = msg.atTime
		}
	case CostStatsMessageFrameTypeEnd:
		if unit.endTime == 0 {
			unit.endTime = msg.atTime
		}
	}
}

func costStatsStart() {
	for {
		select {
		case msg := <-frameStatsChan:
			frameStatsMutex.Lock()
			stream, exist := streams[msg.streamId]
			if !exist {
				stream = streamMap{
					streamId:    msg.streamId,
					frameMap:    make(frameMap),
					frameMinSeq: math.MaxInt64,
					frameMaxSeq: math.MinInt64,
				}
				streams[msg.streamId] = stream
			}

			frame, ok := stream.frameMap[msg.frameSeq]
			if !ok {
				frame = &frameUnit{
					seq:        msg.frameSeq,
					utpMap:     make(map[int64]utpUnit),
					CreateTime: time.Now().UnixMilli(),
				}
				stream.frameMap[msg.frameSeq] = frame
			}
			frameStatsSet(msg, frame)
			stream.frameMaxSeq = int64(math.Max(float64(msg.frameSeq), float64(stream.frameMaxSeq)))
			stream.frameMinSeq = int64(math.Min(float64(msg.frameSeq), float64(stream.frameMinSeq)))
			frameStatsMutex.Unlock()
		}
	}
}

func FrameStatsIncoming(buf []byte) {
	frameStatsChan <- frameStatsMessage{
		frameSeq:  int64(binary.BigEndian.Uint64(buf[0:])),
		atTime:    int64(binary.BigEndian.Uint64(buf[8:])),
		statsType: buf[16],
		streamId:  buf[17],
	}
}

func FrameCostStatsDraw(streamId uint8) ([]byte, error) {
	now := time.Now().UnixMilli()
	frameStatsMutex.Lock()
	stream, exist := streams[streamId]
	if !exist {
		frameStatsMutex.Unlock()
		return nil, fmt.Errorf("Invaild streamId. ")
	}
	for k, v := range stream.frameMap {
		if now-v.CreateTime > conf.StatsWindowsTimeMs && len(stream.frameMap) > conf.StatsWindowsCount {
			delete(stream.frameMap, k)
		} else {
			stream.frameMaxSeq = int64(math.Max(float64(k), float64(stream.frameMaxSeq)))
			stream.frameMinSeq = int64(math.Min(float64(k), float64(stream.frameMinSeq)))
		}
	}
	frameStatsMutex.Unlock()

	var accumulateCost int64 = 0
	validFrame := 0
	frameStatsMutex.RLock()
	xAxis := make([]string, 0)
	YAxis := make([][]opts.LineData, 9)
	for i := stream.frameMinSeq; i <= stream.frameMaxSeq; i++ {
		unit, ok := stream.frameMap[i]
		if !ok {
			continue
		}

		if unit.startTime == 0 || (unit.dropTime == 0 && unit.endTime == 0) {
			continue
		}

		xAxis = append(xAxis, strconv.Itoa(int(unit.seq)))
		YAxis[0] = append(YAxis[0], opts.LineData{Value: 200})

		totalCost := unit.endTime - unit.startTime
		if unit.endTime == 0 && unit.dropTime == 0 {
			if now-unit.sendTime > 3000 {
				totalCost = 0
				YAxis[1] = append(YAxis[1], opts.LineData{Value: totalCost})

				splitFrameCost := unit.enqueueTime - unit.startTime
				YAxis[2] = append(YAxis[2], opts.LineData{Value: splitFrameCost})

				paceQueueCost := unit.dequeueTime - unit.enqueueTime
				YAxis[3] = append(YAxis[3], opts.LineData{Value: paceQueueCost})

				YAxis[4] = append(YAxis[4], opts.LineData{Value: 0})
				YAxis[5] = append(YAxis[5], opts.LineData{Value: 0})
				YAxis[6] = append(YAxis[6], opts.LineData{Value: -100})
				YAxis[7] = append(YAxis[7], opts.LineData{Value: 0})
			}
			continue
		}

		if unit.endTime != 0 {
			YAxis[1] = append(YAxis[1], opts.LineData{Value: totalCost})

			assembleFrameCost := unit.endTime - unit.recvTime
			if assembleFrameCost > 10000 {
				assembleFrameCost = 0
			}
			YAxis[5] = append(YAxis[5], opts.LineData{Value: assembleFrameCost})
			YAxis[6] = append(YAxis[6], opts.LineData{Value: 0})
		} else {
			totalCost = unit.dropTime - unit.startTime
			YAxis[1] = append(YAxis[1], opts.LineData{Value: totalCost})

			assembleFrameCost := unit.dropTime - unit.recvTime
			if assembleFrameCost > 10000 {
				assembleFrameCost = 0
			}
			YAxis[5] = append(YAxis[5], opts.LineData{Value: assembleFrameCost})
			YAxis[6] = append(YAxis[6], opts.LineData{Value: -100})
		}

		if totalCost > 500 {
			log.Printf("TotalCost | [%d] cost [%d]", unit.seq, totalCost)
		}

		if unit.enqueueTime != 0 {
			splitFrameCost := unit.enqueueTime - unit.startTime
			YAxis[2] = append(YAxis[2], opts.LineData{Value: splitFrameCost})
		} else {
			YAxis[2] = append(YAxis[2], opts.LineData{Value: 0})
		}

		paceQueueCost := unit.dequeueTime - unit.enqueueTime
		YAxis[3] = append(YAxis[3], opts.LineData{Value: paceQueueCost})

		netCost := unit.recvTime - unit.sendTime
		if unit.dequeueTime != 0 {
			netCost = unit.recvTime - unit.dequeueTime
			if netCost < 0 && unit.dropTime != 0 {
				netCost = unit.dropTime - unit.dequeueTime
			} else if netCost < 0 && unit.endTime != 0 {
				netCost = unit.endTime - unit.dequeueTime
			} else if netCost < 0 || netCost > totalCost {
				netCost = 0
			}
		}
		if netCost < 0 {
			netCost = 0
		}
		YAxis[4] = append(YAxis[4], opts.LineData{Value: netCost})

		accumulateCost += totalCost
		validFrame++
		averageCost := float64(accumulateCost) / float64(validFrame)
		YAxis[7] = append(YAxis[7], opts.LineData{Value: averageCost})
		//log.Printf("Average | cost [%f] ms", averageCost)
		var lagCost int64 = 0
		if i > stream.frameMinSeq {
			lastUnit, find := stream.frameMap[i-1]
			if find {
				unitEndTime := math.Max(float64(unit.endTime), float64(unit.dropTime))
				lastUnitEndTime := math.Max(float64(lastUnit.endTime), float64(lastUnit.dropTime))
				if lastUnitEndTime != 0 && unitEndTime != 0 {
					lagCost = int64(unitEndTime - lastUnitEndTime)
				}
			}
		}
		YAxis[8] = append(YAxis[8], opts.LineData{Value: lagCost})
	}
	frameStatsMutex.RUnlock()

	var line = charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  types.ThemeShine,
			Width:  "1000px",
			Height: "500px"}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Qos Stats",
			Subtitle: fmt.Sprintf("Stream %d Frame Cost", streamId),
		}),
	)
	line.SetXAxis(xAxis).
		AddSeries("Base", YAxis[0]).
		AddSeries("Total", YAxis[1]).
		AddSeries("SplitFrame", YAxis[2]).
		AddSeries("Queue", YAxis[3]).
		AddSeries("Net", YAxis[4]).
		AddSeries("AssembleFrame", YAxis[5]).
		AddSeries("DropFrame", YAxis[6]).
		AddSeries("Average", YAxis[7]).
		AddSeries("Lag", YAxis[8]).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{Smooth: true, ShowSymbol: false}),
			charts.WithLabelOpts(opts.Label{Show: true}))
	line.Validate()
	return json.Marshal(line.JSON())
}

func CostStatsReset() {
	frameStatsMutex.Lock()
	streams = make(map[uint8]streamMap)
	for len(frameStatsChan) > 0 {
		select {
		case <-frameStatsChan:
		default:
		}
	}
	frameStatsMutex.Unlock()
}
