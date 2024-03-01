package data

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"qos-stats/conf"
)

func NewWsClient() (wsCli *websocket.Conn, err error) {
	url := fmt.Sprintf("wss://%s:%d/Stats?auth=%s",
		conf.TraceDomainName, conf.TracePort, conf.TraceAuth)

	dialer := websocket.Dialer{}
	reqHeader := make(http.Header)
	ws, _, err := dialer.Dial(url, reqHeader)
	if err != nil {
		fmt.Printf("Failed to dial %s, err:%v", url, err)
		return nil, err
	}
	return ws, nil
}

func CloseWsClient(cli *websocket.Conn) {
	if err := cli.Close(); err != nil {
		fmt.Println("close ws client failed, err: ", err)
	}
}
