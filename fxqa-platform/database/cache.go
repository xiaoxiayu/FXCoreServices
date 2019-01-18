package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type CacheHander struct {
	cache_ports []string
	node_ips    []string

	node_hashRing      *Consistent
	cachePort_hashRing *Consistent
}

func (this *CacheHander) Init() {
	this.node_hashRing = NewConsisten()
	this.cachePort_hashRing = NewConsisten()

	for _, node_ip := range this.node_ips {
		this.node_hashRing.Add(node_ip)
	}
	for _, node_ip := range this.cache_ports {
		this.cachePort_hashRing.Add(node_ip)
	}
}

func (this *CacheHander) SetCount(key, val string) error {
	server_ip := this.node_hashRing.Get(key)
	server_port := this.cachePort_hashRing.Get(key)
	url_str := "http://" + server_ip + ":" + server_port + "/data"

	data := make(url.Values)
	data["key"] = []string{"CNT_" + key}
	data["val"] = []string{val}
	fmt.Println("CNT_" + key)
	fmt.Println(val)
	res, err := http.PostForm(url_str, data)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if string(body) != "0" {
		return errors.New(string(body))
	}

	return nil
}

func (this *CacheHander) GetCount(key string) (string, error) {
	server_ip := this.node_hashRing.Get(key)
	server_port := this.cachePort_hashRing.Get(key)

	url_str := "http://" + server_ip + ":" + server_port + "/data?key=CNT_" + key

	client := http.Client{
		Timeout: 6000e9,
	}

	res, err := client.Get(url_str)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if string(body) == "NIL" {
		return "", errors.New("Empty Error")
	}
	fmt.Println(string(body))
	return string(body), nil
}

func (this *CacheHander) SetSelect(key, val, exp string) error {
	server_ip := this.node_hashRing.Get(key)
	server_port := this.cachePort_hashRing.Get(key)

	url_str := "http://" + server_ip + ":" + server_port + "/data"

	data := make(url.Values)
	data["key"] = []string{"SEL_" + key}
	data["val"] = []string{val}
	data["exp"] = []string{exp}
	fmt.Println("SetKey: ", "SEL_"+key)
	res, err := http.PostForm(url_str, data)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if string(body) != "0" {
		return errors.New(string(body))
	}

	return nil
}

func (this *CacheHander) GetSelect(key string) (string, error) {
	server_ip := this.node_hashRing.Get(key)
	server_port := this.cachePort_hashRing.Get(key)

	url_str := "http://" + server_ip + ":" + server_port + "/data?key=SEL_" + key

	client := http.Client{
		Timeout: 6000e9,
	}

	res, err := client.Get(url_str)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if string(body) == "NIL" {
		return "", errors.New("Empty Error")
	}
	fmt.Println("GetKey: ", key)
	return string(body), nil
}
