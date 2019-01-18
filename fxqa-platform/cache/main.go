// Package classification Cache API.
//
// Redis Services.
//
// Terms Of Service:
//
// there are no TOS at this moment, use at your own risk we take no responsibility
//
//     Schemes: http
//     Host: SET:platform
//     BasePath: /platform/cache
//     Version: 0.0.1
//     License: MIT http://opensource.org/licenses/MIT
//     Contact: xiaoxia_yu<xiaoxia_yu@foxitsoftware.com>
//
//     Consumes:
//     - application/json
//     - application/xml
//
//     Produces:
//     - application/json
//     - application/xml
//
//
// swagger:meta
package main

import (
	"flag"
	"fmt"
	"strconv"
	//	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/BurntSushi/toml"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	//	fxqacommon "github.com/xiaoxiayu/services/fxqa-tool/common"
	"log"
	"os"
	"time"

	"runtime"

	"github.com/kardianos/service"
	"github.com/natefinch/lumberjack"
	"gopkg.in/redis.v4"
)

var __port int = 32457
var __version string = "1.0"

type cacheConfig struct {
	Title         string
	Port          int
	EtcdEndpoints string
	Swagger       string
	GatewayCfg    string
	Owner         ownerInfo
	Redis         map[string]redisInfo
	Kubernetes    K8sInfo
	FileServer    string
	//	Test       map[string]testInfo
}

type ownerInfo struct {
	Name string
}

type redisInfo struct {
	Nodelabel  string
	MasterName string
	Port       int
	Password   string
	Db         int
	Addrs      []string
}

type K8sInfo struct {
	Server string
	Port   int
}

func ReadCfg(cfg_paths []string) cacheConfig {
	cfg := flag.String("cfg", "", "Configure file.")

	flag.Parse()
	if *cfg != "" {
		cfg_paths = append(cfg_paths, *cfg)
	}
	var config cacheConfig
	for _, cfg_path := range cfg_paths {
		if _, err := toml.DecodeFile(cfg_path, &config); err != nil {
			//			log.Println(err)
			continue
		}
		//		go fxqacommon.WatcheFile(cfg_path, func() {
		//			var cfg cacheConfig
		//			if _, err := toml.DecodeFile(cfg_path, &cfg); err == nil {
		//				//				service_infos, _ := ParseRouteInfo(cfg.RouteCfg)
		//				//				ParseGateWay(cfg.Kubernetes, service_infos)
		//			} else {
		//				//log.Println(err)
		//			}
		//		})
		break
	}

	return config
}

var memprofile = flag.String("memprofile", "", "write memory profile to this file")

var router = mux.NewRouter()

type CacheRequestHandler struct {
	k8s_nodes map[string][]string
	// Master for write.
	master_clients  map[string]*redis.Client
	master_hashRing *Consistent
}

type ServerCFG struct {
	RedisIp string

	ListenPort int
}

// swagger:route GET /info Cache info
//
// Lists pets filtered by some parameters.
//
// This will show all available pets by default.
// You can get the pets that are out of stock
//
//     Consumes:
//     - application/json
//     - application/x-protobuf
//
//     Produces:
//     - application/json
//     - application/x-protobuf
//
//     Schemes: http, https, ws, wss
//
//     Security:
//       api_key:
//       oauth: read, write
//
//     Responses:
//       default: genericError
//       200: someResponse
//       422: validationError
func Info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%v", "Cache Server Running...")
}

var G_FILE_SERVER string

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// k8s service port: 32457
type program struct {
	logger *log.Logger
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

func (p *program) run() {

	cfg := ReadCfg([]string{"/etc/fxqa-cache.conf", "fxqa-cache.conf"})
	G_FILE_SERVER = "http://" + cfg.FileServer

	request_serv := new(CacheRequestHandler)
	err := request_serv.Init(cfg)
	if err != nil {
		fmt.Println("Redis Link Error.")
		return
	}
	router.HandleFunc("/info", Info).Methods("GET")

	router.HandleFunc("/key/{key}", request_serv.delKey).Methods("DELETE")
	router.HandleFunc("/key/{key}", request_serv.updateKey).Methods("PUT")
	router.HandleFunc("/key", request_serv.getKey).Methods("GET")

	router.HandleFunc("/string", request_serv.setString).Methods("POST")
	router.HandleFunc("/string/{key}", request_serv.updateString).Methods("PUT")
	router.HandleFunc("/string", request_serv.getString).Methods("GET")

	// curl -d "key=test&v0 0 v1 1" /hash
	router.HandleFunc("/hash", request_serv.setHash).Methods("POST")
	router.HandleFunc("/hash", request_serv.getHash).Methods("GET")
	router.HandleFunc("/hash/{key:.*}", request_serv.updateHash).Methods("PUT")
	router.HandleFunc("/hash/{key}/{field}", request_serv.delHash).Methods("DELETE")

	router.HandleFunc("/set", request_serv.setSet).Methods("POST")
	router.HandleFunc("/set", request_serv.getSet).Methods("GET")
	router.HandleFunc("/set/{key0}/{key1}", request_serv.updateSet).Methods("PUT")
	router.HandleFunc("/set/{key0}", request_serv.updateSet).Methods("PUT")
	router.HandleFunc("/set/{key}", request_serv.delSet).Methods("DELETE")

	router.HandleFunc("/zset", request_serv.setZset).Methods("POST")
	router.HandleFunc("/zset", request_serv.getZset).Methods("GET")
	router.HandleFunc("/list", request_serv.setList).Methods("POST")

	// curl /hash?key=test | /hash?key=test&field=v0

	//	router.HandleFunc("/list", request_serv.getList).Methods("GET")
	//	router.HandleFunc("/data", request_serv.Get).Methods("GET")
	router.HandleFunc("/db", request_serv.RedisDBGet).Methods("GET")

	router.HandleFunc("/server", request_serv.ServerAdd).Methods("POST")
	router.HandleFunc("/server", request_serv.ServerGet).Methods("GET")

	//router.HandleFunc("/del", request_serv.del).Methods("DELETE")

	// Later.
	router.HandleFunc("/sync", request_serv.RedisSync).Methods("POST")

	http.Handle("/", router)
	fmt.Println("start:", __port)
	http.ListenAndServe(":"+strconv.Itoa(__port), handlers.CORS()(router))
}

func main() {
	var err error
	logpath := "/var/log/fxqa/fxqa_cache.log"
	if runtime.GOOS == "windows" {
		logpath = "fxqa_cache.log"
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

	portPtr := flag.Int("port", 32457, "port")

	flag.Parse()
	__port = *portPtr

	//	__heartbeat_info = cmap.New()

	serviceName := "FoxitQARedisCache"
	displayName := "Foxit QA Redis Cache Service"
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: displayName,
		Description: "Foxit QA Redis Cache Service.",
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
