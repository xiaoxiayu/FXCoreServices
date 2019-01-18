package main

import (
	"fmt"
	"net/url"
	"testing"
	"time"
)

func ContinueConcurrence(testFunc func(chan int), con_num int) {
	chs := make(chan int, con_num)

	go func() {
		for j := 0; j < int(con_num); j++ {
			go testFunc(chs)
		}

	}()

	for true {
		select {
		case <-chs:
			//time.Sleep(1e3)
			go testFunc(chs)
		}
	}
	return
}

func PostLogStr(chs chan int) {
	url_str := "http://10.103.129.82:9091/logs"
	logdata := make(url.Values)
	t := time.Now()

	log_str := t.Format("2006-01-02 15:04:05")
	log_str += "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	logdata["s"] = []string{string(log_str)}
	logdata["n"] = []string{string("test")}
	_, err := HTTPPost(url_str, logdata)
	if err != nil {
		fmt.Println(err.Error())
	}
	chs <- 1
}

func Test_Write(t *testing.T) {
	ContinueConcurrence(PostLogStr, 1000)
}
