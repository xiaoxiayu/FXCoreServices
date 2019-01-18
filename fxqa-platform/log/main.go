package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strconv"

	"os"

	"github.com/BurntSushi/toml"
	"github.com/gorilla/mux"
	"github.com/kardianos/service"
	//"github.com/gorilla/securecookie"
)

var __version string = "1.0"

type logConfig struct {
	Title   string
	Owner   ownerInfo
	Service serviceInfo
	Log     map[string]logInfo
}

type ownerInfo struct {
	Name string
}

type serviceInfo struct {
	Port int
}

type logInfo struct {
	Path      string
	MaxSize   int
	MaxAge    int
	MaxBackup int
	Enable    bool
}

func ReadCfg(cfg_paths []string) logConfig {
	//	cfg_path := "/etc/fxqa-log.conf"
	cfg := flag.String("cfg", "", "Configure file.")
	flag.Parse()
	if *cfg != "" {
		cfg_paths = append(cfg_paths, *cfg)
	}
	var config logConfig

	for _, cfg_path := range cfg_paths {
		if _, err := toml.DecodeFile(cfg_path, &config); err != nil {
			//			log.Println(err)
			continue
		}
		fmt.Println(cfg_path)
		break
	}

	return config
}

var memprofile = flag.String("memprofile", "", "write memory profile to this file")

var router = mux.NewRouter()

type ToolService struct {
}

type LogService struct {
	cfg           logConfig
	logFileHandle map[string]*log.Logger
}

type TestFxcore struct {
	logServer string
}

func Info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%v", "FoxitQA Log Server Running...")
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

type program struct {
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *program) run() {
	cfg := ReadCfg([]string{"/etc/fxqa-log.conf", "fxqa-log.conf"})
	fmt.Println("START")

	log_serv := LogService{cfg: cfg}
	log_serv.Init()

	router.HandleFunc("/info", Info).Methods("GET")

	router.HandleFunc("/logs", log_serv.LogWrite).Methods("POST")
	router.HandleFunc("/logs/register", log_serv.LogRegister).Methods("POST")

	http.Handle("/", router)
	http.ListenAndServe(":"+strconv.Itoa(cfg.Service.Port), nil)

}
func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

func main() {
	serviceName := "FoxitQALogService"
	displayName := "Foxit QA Log Service"
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: displayName,
		Description: "Foxit QA Log Service.",
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
