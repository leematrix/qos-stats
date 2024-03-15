package biz

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	TypeBweStats        = 0
	TypeTransportStats  = 1
	TypeFrameStats      = 2
	TypeDelayTrendStats = 3
	TypeNSEStats        = 4
	TypeReset           = 255
)

type wsStatsReqMessage struct {
	StatsType string `json:"statsType"`
	StreamId  int    `json:"streamId"`
}

type wsStatsRespMessage struct {
	StatsType string `json:"statsType"`
	StreamId  int    `json:"streamId"`
	Payload   string `json:"payload"`

	Success bool `json:"success"` // 标志WebSocket请求是否成功，仅给Web客户端回复时有效
}

func WsStats(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		Subprotocols: []string{"json"},
		CheckOrigin: func(r *http.Request) bool {
			return true
		}, // 关闭请求源地址检查
	}

	// 升级连接协议到WebSocket
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade to websocket fail: %s", err.Error())
		return
	}
	defer c.Close()

	var mutex sync.Mutex
	respFun := func(statsType string, streamId int, payload string, success bool) {
		resp := wsStatsRespMessage{
			StatsType: statsType,
			StreamId:  streamId,
			Payload:   payload,
			Success:   success,
		}

		mutex.Lock()
		if result, resErr := json.Marshal(resp); resErr == nil {
			err = c.WriteMessage(1, result)
			if err != nil {
				log.Println("respErr", err)
			}
		} else {
			log.Println("resErr", resErr)
		}
		mutex.Unlock()
	}

	log.Printf("New ws client, addr: %s", r.RemoteAddr)

	isExit := false
	go func() {
		for !isExit {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := BweStatsDraw()
				respFun("bwe", -1, string(respData[:]), respErr == nil)
			}
		}
	}()

	//
	//go func() {
	//	for !isExit {
	//		ticker := time.NewTicker(500 * time.Millisecond)
	//		select {
	//		case <-ticker.C:
	//			respData, respErr := TrendStatsDraw()
	//			respFun("trend", -1, string(respData[:]), respErr == nil)
	//		}
	//	}
	//}()
	//
	//go func() {
	//	for !isExit {
	//		ticker := time.NewTicker(500 * time.Millisecond)
	//		select {
	//		case <-ticker.C:
	//			respData, respErr := FrameCostStatsDraw(0)
	//			respFun("cost", 0, string(respData[:]), respErr == nil)
	//			respData, respErr = FrameCostStatsDraw(1)
	//			respFun("cost", 1, string(respData[:]), respErr == nil)
	//		}
	//	}
	//}()
	//
	//go func() {
	//	for !isExit {
	//		ticker := time.NewTicker(500 * time.Millisecond)
	//		select {
	//		case <-ticker.C:
	//			respData, respErr := FrameCostStatsDraw(1)
	//			respFun("cost", 1, string(respData[:]), respErr == nil)
	//		}
	//	}
	//}()

	go func() {
		for !isExit {
			ticker := time.NewTicker(500 * time.Millisecond)
			select {
			case <-ticker.C:
				respData, respErr := NseStatsDraw()
				respFun("nse", 255, string(respData[:]), respErr == nil)
			}
		}
	}()

	// 从WebSocket连接轮询消息
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			isExit = true
			log.Printf("Ws read fail: %s", err.Error())
			break
		}

		log.Printf("Ws recv: %s", string(message))

		var statsMsg = wsStatsReqMessage{}
		err = json.Unmarshal(message, &statsMsg)
		if err != nil {
			log.Printf("Unmarshal fail: %s", err.Error())
			return
		}
		var respErr error = nil
		var respData []byte
		switch statsMsg.StatsType {
		case "bwe-all":
			respData, respErr = BweStatsDraw()
			respFun("bwe", -1, string(respData[:]), respErr == nil)
			respData, respErr = TrendStatsDraw()
			respFun("trend", -1, string(respData[:]), respErr == nil)
			respData, respErr = FrameCostStatsDraw(0)
			respFun("cost", 0, string(respData[:]), respErr == nil)
			respData, respErr = FrameCostStatsDraw(1)
			respFun("cost", 1, string(respData[:]), respErr == nil)
		case "bwe":
			respData, respErr = BweStatsDraw()
			respFun("bwe", -1, string(respData[:]), respErr == nil)
		case "trend":
			respData, respErr = TrendStatsDraw()
			respFun("trend", -1, string(respData[:]), respErr == nil)
		case "cost":
			respData, respErr = FrameCostStatsDraw(uint8(statsMsg.StreamId))
			respFun("cost", statsMsg.StreamId, string(respData[:]), respErr == nil)
		case "nse":
			respData, respErr = NseStatsDraw()
			respFun("nse", 255, string(respData[:]), respErr == nil)
		}

		if respErr != nil {
			log.Println("respErr", respErr)
		}
	}
}

type tcMessage struct {
	RealBandwidth int `json:"realBandwidth"`
	RecvQueueLen  int `json:"recvQueueLen"`
}

func WsTc(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		Subprotocols: []string{"json"},
		CheckOrigin: func(r *http.Request) bool {
			return true
		}, // 关闭请求源地址检查
	}

	// 升级连接协议到WebSocket
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade to websocket fail: %s", err.Error())
		return
	}
	defer c.Close()

	// 从WebSocket连接轮询消息
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("Ws read fail: %s", err.Error())
			break
		}

		log.Printf("Ws recv: %s", string(message))

		var tcMsg = tcMessage{}
		err = json.Unmarshal(message, &tcMsg)
		if err != nil {
			continue
		}
		realBandwidthKBPS = tcMsg.RealBandwidth
		recvQueueLen = tcMsg.RecvQueueLen
	}
}

func Start() {
	go TraceStart()
	go bweStatsStart()
	go costStatsStart()
	go trendStatsStart()
	go nseStatsStart()
}
