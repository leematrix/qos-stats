package main

import (
	"io"
	"log"
	"os"
	"qos-stats/biz"
	"qos-stats/conf"
	"qos-stats/server"
)

var file *os.File

func logInit() {
	if !conf.OpenLog {
		return
	}

	var err error
	file, err = os.OpenFile("qos_stats.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
	log.Println("Qos Stats Start.")

	logInit()
	biz.Start()
	server.SignalServerStart()
	server.HttpStart()
	log.Println("Qos Stats Exit.")
}
