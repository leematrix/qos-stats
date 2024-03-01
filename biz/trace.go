package biz

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"qos-stats/data"
)

var wsCli *websocket.Conn
var nseStatsMsgChan = make(chan NseStats, 1024)

func TraceStart() {
	ws, err := data.NewWsClient()
	if err != nil {
		fmt.Println("New ws client failed, err: ", err)
	} else {
		wsCli = ws
	}

	for {
		select {
		case msg := <-nseStatsMsgChan:
			marshal, _ := json.Marshal(msg)
			if err = wsCli.WriteMessage(websocket.BinaryMessage, marshal); err != nil {
				fmt.Println("send ws msg failed, err:", err)
				wsCli, _ = data.NewWsClient()
			}
			fmt.Println("send ws msg ok.")
		}
	}
}

func TraceIncoming(msg NseStats) {
	select {
	case nseStatsMsgChan <- msg:
	default:
	}
}