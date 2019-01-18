package main

import (
	//	"fmt"
	"log"
	"strings"
	"time"

	"encoding/json"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// workerInfo is the service register information to etcd
type WorkerInfo struct {
	Name string
	IP   string
	CPU  int
}

type DiscoveryMaster struct {
	members map[string]*Member
	KeysAPI client.KeysAPI
}

// Member is a client machine
type Member struct {
	InGroup bool
	IP      string
	Name    string
	CPU     int
}

func NewMaster(endpoints []string) *DiscoveryMaster {
	cfg := client.Config{
		Endpoints:               endpoints,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}

	etcdClient, err := client.New(cfg)
	if err != nil {
		log.Fatal("Error: cannot connec to etcd:", err)
	}

	master := &DiscoveryMaster{
		members: make(map[string]*Member),
		KeysAPI: client.NewKeysAPI(etcdClient),
	}
	go master.WatchWorkers()
	return master
}

func (m *DiscoveryMaster) AddWorker(info *WorkerInfo) {
	member := &Member{
		InGroup: true,
		IP:      info.IP,
		Name:    info.Name,
		CPU:     info.CPU,
	}
	m.members[member.Name] = member
}

func (m *DiscoveryMaster) UpdateWorker(info *WorkerInfo) {
	member := m.members[info.Name]
	member.InGroup = true
}

func (m *DiscoveryMaster) WatchWorkers() {
	api := m.KeysAPI
	watcher := api.Watcher("workers/", &client.WatcherOptions{
		Recursive: true,
	})
	for {
		res, err := watcher.Next(context.Background())
		if err != nil {
			log.Println("Error watch workers:", err)
			break
		}
		if res.Action == "expire" {
			//fmt.Println("==================EXPIRE==============", res.Node.Key)
			node_names := strings.Split(res.Node.Key, "/workers/")
			//fmt.Println(node_names[1])
			member, ok := m.members[node_names[1]]
			if ok {
				member.InGroup = false
			}
		} else if res.Action == "set" || res.Action == "update" {

			info := &WorkerInfo{}
			err := json.Unmarshal([]byte(res.Node.Value), info)
			if err != nil {
				log.Print(err)
			}
			//fmt.Println("=========SET:", info.Name)
			if _, ok := m.members[info.Name]; ok {
				m.UpdateWorker(info)
			} else {
				m.AddWorker(info)
			}
			//			fmt.Println("===============SET=============", info)
			//fmt.Println(m.members)
		} else if res.Action == "delete" {
			delete(m.members, res.Node.Key)
			//fmt.Println("=======DELETE=========")
		}
	}

}

func (m *DiscoveryMaster) GetOnlineNode(nodes_key []string) []string {
	nodes_ip := []string{}
	for _, mem := range m.members {
		if !mem.InGroup {
			continue
		}

		for _, node_k := range nodes_key {
			if 0 == strings.Index(mem.Name, node_k) {
				nodes_ip = append(nodes_ip, mem.IP)
				break
			}
		}
	}
	//fmt.Println("NODES_IP:", nodes_ip)
	return nodes_ip
}
