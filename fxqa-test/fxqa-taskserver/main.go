package main

import (
	"flag"

	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"strconv"

	"sync"

	"time"

	"github.com/gorilla/mux"

	"github.com/gorilla/handlers"
	"github.com/kardianos/service"
	cmap "github.com/streamrail/concurrent-map"
)

var __logserver string = "10.103.129.82:9090"
var __version string = "2.4"

type HeartbeatData struct {
	Ip          string `json:"ip"`
	MachineInfo string `json:"machineinfo"`
	Status      string `json:"status"` // free or busy
	TestInfo    string `json:"testinfo"`
	TimeStamp   int64  `json:"time"`

	Label      string `json:"label"`
	Concurrent int    `json:"concurrent"`
}

type TestData struct {
	Ip      string `json:"ip"`
	TaskCnt int    `json:"taskcount"`
	Label   string `json:label`
}

var router = mux.NewRouter()

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

type retData struct {
	retChan  chan string
	taskIp   string
	taskData string
}

type ManagerService struct {
	//	cfg            managerConfig
	WorkerMachines []string
	//	DisMaster      *DiscoveryMaster

	Locker *sync.Mutex
	//NodesIP []string

	logfile *os.File
	logger  *log.Logger

	//TaskConcurrent int
	TaskConcurrent cmap.ConcurrentMap
	//TaskChan       chan retData
	WSClientMap cmap.ConcurrentMap //map[string]*websocket.Conn

	//	MachineInfo cmap.ConcurrentMap
	//TaskCount   cmap.ConcurrentMap

	//TaskCountChan chan retData
}

type serverInfo struct {
	Server  string
	Port    int
	Enabled bool
}

func Version(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%v", fmt.Sprintf(`{"version":"%s"}`, __version))
}

type program struct {
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *program) run() {
	// Do work here
	portPtr := flag.Int("port", 9090, "port")
	flag.Parse()

	mgr_serv := ManagerService{
		Locker:         new(sync.Mutex),
		TaskConcurrent: cmap.New(),
		WSClientMap:    cmap.New(),
	}
	mgr_serv.Init()

	router.HandleFunc("/version", Version).Methods("GET")
	router.HandleFunc("/reset", mgr_serv.Reset).Methods("POST")
	router.HandleFunc("/fxcore", mgr_serv.testStart).Methods("POST")
	router.HandleFunc("/testserver", mgr_serv.GetTestServer).Methods("GET")
	router.HandleFunc("/testserver", mgr_serv.UpdateTestServer).Methods("PUT")
	//	router.HandleFunc("/testversion", mgr_serv.setTestVersion).Methods("POST")

	router.HandleFunc("/concurrent", mgr_serv.SetConcurrent).Methods("POST")
	router.HandleFunc("/concurrent", mgr_serv.GetConcurrent).Methods("GET")

	router.HandleFunc("/info", mgr_serv.Info).Methods("GET")

	router.HandleFunc("/fxcorekl", mgr_serv.testStartKeepalive)

	http.Handle("/", router)
	fmt.Println("START")

	methods := handlers.AllowedMethods(
		[]string{"DELETE", "GET", "HEAD", "POST", "PUT", "OPTIONS"})
	http.ListenAndServe(":"+strconv.Itoa(*portPtr), handlers.CORS(methods)(router))

}
func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

func main() {
	serviceName := "FoxitQACoreManager"
	displayName := "Foxit QA Core Manager Service"
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: displayName,
		Description: "Foxit QA Core Manager Service.",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		//		logger.Fatal(err)
		return
	}

	if len(os.Args) > 1 {
		var err error
		verb := os.Args[1]
		switch verb {
		case "install":
			err = s.Install()
			if err != nil {
				fmt.Printf("Failed to install: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" installed.\n", displayName)
		case "start":
			err = s.Start()
			if err != nil {
				fmt.Printf("Failed to start: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" started.\n", displayName)
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				fmt.Printf("Failed to uninstall: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" uninstalled.\n", displayName)
		case "stop":
			err = s.Stop()
			if err != nil {
				fmt.Printf("Failed to stop: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" stopped.\n", displayName)
		case "restart":
			err = s.Stop()
			if err != nil {
				fmt.Printf("Failed to restart: %s\n", err)
				return
			}
			err = s.Start()
			if err != nil {
				fmt.Printf("Failed to restart: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" restart.\n", displayName)
		case "version":
			fmt.Println(__version)
		default:
			s.Run()
		}
		return
	}

	err = s.Run()
	if err != nil {
		//	s.Error(err.Error())
	}
}
