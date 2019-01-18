package main

import (
	"flag"
	//	"io"
	"io/ioutil"
	"path/filepath"
	"sync"

	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/kardianos/service"

	"strconv"
	"strings"

	"encoding/json"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/natefinch/lumberjack"

	cmap "github.com/streamrail/concurrent-map"
)

var __version string = "2.51"
var __task_server string = ""
var __task_ws_path string = "/fxcorekl"
var __port int = 9091
var __cfgpath string = ""
var __program_path string = ""

var __heartbeat_info cmap.ConcurrentMap
var __heartbeat_url string = "http://10.103.129.79/test/state/heartbeat"

var __machine_task_cnt int = 0

var __test_is_online bool
var __machine_label string = ""
var __machine_info string = "Unknown"
var G_LOCALIP = ""
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

var router = mux.NewRouter()

type LogService struct {
	logPath       string
	logFileHandle map[string]*os.File
}

type TestService struct {
	send chan []byte

	logfile *os.File
	logger  *log.Logger
	wsconn  *websocket.Conn

	LinkRetryIndex    int
	TaskCntLocker     *sync.RWMutex
	fuzzoutput_folder cmap.ConcurrentMap
}

func GetEnv() map[string]string {
	getenvironment := func(data []string, getkeyval func(item string) (key, val string)) map[string]string {
		items := make(map[string]string)
		for _, item := range data {
			key, val := getkeyval(item)
			//			fmt.Println(key)
			items[key] = val
		}
		return items
	}
	environment := getenvironment(os.Environ(), func(item string) (key, val string) {
		splits := strings.Split(item, "=")
		key = splits[0]
		val = splits[1]
		return
	})

	return environment
}

func Version(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "%v", fmt.Sprintf(`{"version":%s,"ip":"%s"}`, __version, GetEnv()["FXIP"]))
}

type program struct {
	logger *log.Logger
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

type CfgData struct {
	Ip           string `json:"ip"`
	TaskCnt      int    `json:"taskcount"`
	Label        string `json:"label"`
	ProgramPath  string `json:"program"`
	TaskServer   string `json:"taskserver"`
	HeartBeatUrl string `json:"heartbeat_url"`
}

func initConfig(logger *log.Logger) {
	__test_is_online = false
	var localIP string
	if runtime.GOOS == "windows" {
		__cfgpath = "fxqa_testserver_" + strconv.Itoa(__port) + ".cfg"
	} else {
		__cfgpath = "/foxitqa/fxqa_testserver_" + strconv.Itoa(__port) + ".cfg"
	}

	if _, err := os.Stat(__cfgpath); err == nil {
		data, err := ioutil.ReadFile(__cfgpath)
		if err == nil {
			var cfgdata CfgData
			err = json.Unmarshal(data, &cfgdata)
			if err != nil {
				return
			}
			G_LOCALIP = cfgdata.Ip
			__machine_label = cfgdata.Label
			__task_server = cfgdata.TaskServer
			__program_path = cfgdata.ProgramPath
			__heartbeat_url = cfgdata.HeartBeatUrl

			return
		}
	}
	localIP = GetEnv()["FXIP"]
	if localIP != "" {
		G_LOCALIP = localIP + ":" + strconv.Itoa(__port)
	}

	if runtime.GOOS == "windows" {
		//		localIP = GetEnv()["FXIP"]
		//		G_LOCALIP = localIP + ":" + strconv.Itoa(__port)
	} else {
		ip, _ := GetLocalIP("")
		localIP = ip.String()
	}
	if localIP == "" {
		fmt.Println("Get FXIP From Environment ERROR.")
		logger.Println("Get FXIP From Environment ERROR.")
		os.Exit(1)
	}
	G_LOCALIP = localIP + ":" + strconv.Itoa(__port)

}

func (p *program) run() {
	initConfig(p.logger)

	test_serv := TestService{logger: p.logger,
		LinkRetryIndex:    0,
		fuzzoutput_folder: cmap.New(),
		TaskCntLocker:     new(sync.RWMutex)}

	logdir, _ := filepath.Abs("./")
	if runtime.GOOS != "windows" {
		logdir = "/var/log/fxqa/"
	}

	router.PathPrefix("/fxqalog/").Handler(
		http.StripPrefix("/fxqalog", http.FileServer(http.Dir(logdir))))

	router.HandleFunc("/version", Version).Methods("GET")

	router.HandleFunc("/init", test_serv.InitParam).Methods("POST")
	router.HandleFunc("/reset", test_serv.Reset).Methods("GET")

	router.HandleFunc("/info", test_serv.Info).Methods("GET")
	router.HandleFunc("/label", test_serv.GetLabel).Methods("GET")
	router.HandleFunc("/label", test_serv.SetLabel).Methods("PUT")
	router.HandleFunc("/label", test_serv.DelLabel).Methods("DELETE")

	router.HandleFunc("/fxcore", test_serv.fxcoreTest).Methods("POST")
	router.HandleFunc("/sdk", test_serv.fxcoreTest).Methods("POST")

	router.HandleFunc("/update", test_serv.Update).Methods("POST")

	router.HandleFunc("/upload", test_serv.Upload).Methods("POST")

	router.HandleFunc("/kill", test_serv.Kill).Methods("POST")
	router.HandleFunc("/cmd", test_serv.CmdRun).Methods("POST")
	router.HandleFunc("/log", test_serv.ClearLog).Methods("DELETE")

	router.HandleFunc("/online", test_serv.GetOnline).Methods("GET")
	router.HandleFunc("/online", test_serv.SetOnline).Methods("POST")

	router.HandleFunc("/hardware", test_serv.GetHardwareInfo).Methods("GET")

	router.HandleFunc("/fuzz-data", test_serv.GetFuzzData).Methods("GET")
	router.HandleFunc("/fuzz-data", test_serv.SetFuzzData).Methods("POST")
	router.HandleFunc("/fuzz-data/{name}", test_serv.DelFuzzData).Methods("DELETE")

	http.Handle("/", router)

	//	__heartbeat_info = cmap.New()
	//	__heartbeat_info.Set("status", "available")

	//	if __heartbeat_url == "" {
	//		__heartbeat_url = "http://10.103.129.79/test/state/heartbeat"
	//	}
	//	go HeartBeatStart("FXCORE", G_LOCALIP, 5)
	go test_serv.fxcoreLKStart()

	fmt.Println("START:", G_LOCALIP)

	methods := handlers.AllowedMethods(
		[]string{"DELETE", "GET", "HEAD", "POST", "PUT", "OPTIONS"})
	http.ListenAndServe(":"+strconv.Itoa(__port), handlers.CORS(methods)(router))
	//http.ListenAndServe(":"+os.Args[1], nil)
}
func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

func main() {
	var err error
	logpath := "/var/log/fxqa/test.log"
	if runtime.GOOS == "windows" {
		logpath = "test.log"
	} else {
		ret, _ := exists("/var/log/fxqa")
		if !ret {
			os.MkdirAll("/var/log/fxqa", 0777)
		}
	}
	fmt.Println("LogPath:", logpath)
	logger := log.New(nil, "", log.Ldate|log.Ltime|log.Lshortfile)
	logger.SetOutput(&lumberjack.Logger{
		Filename:   logpath,
		MaxSize:    200, // megabytes
		MaxBackups: 3,
		MaxAge:     15, //days
	})

	logger.Println(`Log Start:`,
		time.Now().Format("2006-01-02 15:04:05"))
	_, err = os.Stat(logpath)
	if err != nil && os.IsNotExist(err) {
		log.Panic("Error: can not create logfile:", logpath)
	}

	portPtr := flag.Int("port", 9091, "port")

	flag.Parse()
	__port = *portPtr

	//	__heartbeat_info = cmap.New()

	serviceName := "FoxitQACoreTester"
	displayName := "Foxit QA Core Test Service"
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: displayName,
		Description: "Foxit QA Core Test Service.",
	}

	prg := &program{logger: logger}
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
