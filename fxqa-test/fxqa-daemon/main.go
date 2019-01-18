package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	//	"syscall"
	"time"

	"log"
	"runtime"

	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"

	fxqacommon "foxitsoftware.cn/quality_control/services/fxqa-tool/common"
	"github.com/kardianos/service"
	"github.com/natefinch/lumberjack"
)

var __version string = "1.0"

var __update_info_url string = "http://10.103.2.166:9090/_qaupdater_info/_core_version_data"

var __b_client bool = true
var __b_taskserver bool = false

//var router = mux.NewRouter()
var G_LOCALIP = ""

//cgui_serv := new(CGUIDaemonService)

type VersionData struct {
	LastestVersion float32 `json:"lastest_version"`
	LastestUrl     string  `json:"lastest_url"`
}

type PlatformData struct {
	Darwin  VersionData `json:"darwin"`
	Linux32 VersionData `json:"linux32"`
	Linux64 VersionData `json:"linux64"`
	Windows VersionData `json:"windows"`
}

type CoreData struct {
	Client     PlatformData `json:"client"`
	TaskServer PlatformData `json:"task_server"`
}

type LocalData struct {
	Version    float32 `json:"version"`
	Wspath     string  `json:"wspath"`
	Program    string  `json:"program"`
	Taskserver string  `json:"taskserver"`
	Label      string  `json:"label"`
	ExecPath   string  `json:"exec"`
}

type ResData struct {
	Res int `json:"ret"`
}

type program struct {
	logger *log.Logger
}

func GetLocalClientData() (LocalData, error) {
	var localClientdata LocalData
	res, err := fxqacommon.HTTPGet("http://127.0.0.1:9091/info")
	if err != nil {
		fmt.Println(err.Error())
		return localClientdata, err
	}
	fmt.Println(string(res))

	err = json.Unmarshal(res, &localClientdata)
	if err != nil {
		fmt.Println(err.Error())
		return localClientdata, err
	}

	return localClientdata, nil
}

func InitClient(localClientdata LocalData) (ret bool, err error) {
	data := make(url.Values)
	data["program"] = []string{string(localClientdata.Program)}
	data["wspath"] = []string{string(localClientdata.Wspath)}
	data["taskserver"] = []string{string(localClientdata.Taskserver)}
	data["label"] = []string{string(localClientdata.Label)}
	init_url := "http://127.0.0.1:9091/init"

	res, err := fxqacommon.HTTPPost(init_url, data)
	if err != nil {
		err = fmt.Errorf(err.Error())
		return
	}
	var resData ResData
	err = json.Unmarshal([]byte(res), &resData)
	if err != nil {
		ret = false
		return
	}
	if resData.Res != 0 {
		ret = false
	}
	ret = true
	return
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

func (p *program) run() {

	var err error
	var localClientdata LocalData

	for true {
		localClientdata, err = GetLocalClientData()
		if err != nil {
			p.logger.Println(fmt.Sprintf(`get local client data error:%v`, err.Error()),
				time.Now().Format("2006-01-02 15:04:05"))
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}

	var update_time time.Duration = 30
	for true {
		res, err := fxqacommon.HTTPGet(__update_info_url)
		if err != nil {
			p.logger.Println(fmt.Sprintf(`get version data error:%v`, err.Error()),
				time.Now().Format("2006-01-02 15:04:05"))

			time.Sleep(update_time * time.Second)
			continue
		}
		var updateData CoreData
		err = json.Unmarshal(res, &updateData)
		if err != nil {
			p.logger.Println(fmt.Sprintf(`parse update data error:%v`, err.Error()),
				time.Now().Format("2006-01-02 15:04:05"))
			time.Sleep(update_time * time.Second)
			continue
		}

		var remote_data_version float32
		var remote_data_url string
		if runtime.GOOS == "darwin" {
			remote_data_version = updateData.Client.Darwin.LastestVersion
			remote_data_url = updateData.Client.Darwin.LastestUrl
		} else if runtime.GOOS == "windows" {
			remote_data_version = updateData.Client.Windows.LastestVersion
			remote_data_url = updateData.Client.Windows.LastestUrl
		} else if runtime.GOOS == "linux" {
			if runtime.GOARCH == "386" {
				remote_data_version = updateData.Client.Linux32.LastestVersion
				remote_data_url = updateData.Client.Linux32.LastestUrl
			} else {
				remote_data_version = updateData.Client.Linux64.LastestVersion
				remote_data_url = updateData.Client.Linux64.LastestUrl
			}
		}
		if remote_data_version != localClientdata.Version {
			p.logger.Println(
				time.Now().Format("2006-01-02 15:04:05"),
				fmt.Sprintf(`update client:%v, new version:%v`,
					string(remote_data_url),
					remote_data_version))
			_, err := http.Head(remote_data_url)
			if err != nil {
				p.logger.Println(fmt.Sprintf(`%v: %v`, remote_data_url, err.Error()),
					time.Now().Format("2006-01-02 15:04:05"))
				time.Sleep(update_time * time.Second)
				continue
			}

			// Get Info before restart
			var preinfo LocalData
			for i := 0; i < 120; i++ {
				preinfo, err = GetLocalClientData()
				if err != nil {
					p.logger.Println(fmt.Sprintf(`get local client data error:%v`, err.Error()),
						time.Now().Format("2006-01-02 15:04:05"))
					time.Sleep(1 * time.Second)
					continue
				}
			}

			data := make(url.Values)
			data["url"] = []string{string(remote_data_url)}
			update_url := "http://127.0.0.1:9091/update"

			res, err := fxqacommon.HTTPPost(update_url, data)
			p.logger.Println(
				time.Now().Format("2006-01-02 15:04:05"),
				`trigger client to update`)
			if err != nil {
				p.logger.Println(fmt.Sprintf(`update client error:%v`, err.Error()),
					time.Now().Format("2006-01-02 15:04:05"))
				time.Sleep(update_time * time.Second)
				continue
			}
			var resData ResData
			err = json.Unmarshal([]byte(res), &resData)
			if err != nil {
				p.logger.Println(time.Now().Format("2006-01-02 15:04:05"),
					fmt.Sprintf(`parse client updated respond error:%s`, err.Error()))
				time.Sleep(update_time * time.Second)
				continue
			}
			if resData.Res != 0 {
				p.logger.Println(time.Now().Format("2006-01-02 15:04:05"),
					fmt.Sprintf(`get client respond error:%s`, err.Error()))
				time.Sleep(update_time * time.Second)
				continue
			}
			// Restart Client
			p.logger.Println(
				time.Now().Format("2006-01-02 15:04:05"),
				fmt.Sprintf(`restart client:%v`,
					localClientdata.Version))

			var cmd *exec.Cmd
			cmd_list := []string{}
			if runtime.GOOS == "windows" {
				exitdata := make(url.Values)
				fxqacommon.HTTPPost("http://127.0.0.1:9091/_exit", exitdata)

				cmd_list = append(cmd_list, "cmd.exe")
				cmd_list = append(cmd_list, "/c")
				cmd_list = append(cmd_list, "call")
				cmd_list = append(cmd_list, string(localClientdata.ExecPath))
				cmd = exec.Command(cmd_list[0], cmd_list[1:]...)
				//				cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			} else {
				cmd_list = append(cmd_list, "/foxitqa/fxqa-testserver")
				cmd_list = append(cmd_list, "restart")
				cmd = exec.Command(cmd_list[0], cmd_list[1:]...)

			}

			err = cmd.Start()
			if err != nil {
				p.logger.Println(
					time.Now().Format("2006-01-02 15:04:05"),
					fmt.Sprintf(`Start Client Error:%v`, err.Error()))
				time.Sleep(update_time * time.Second)
				continue
			}

			b_success := false
			for i := 0; i < 120; i++ {
				localClientdata, _ = GetLocalClientData()

				if localClientdata.Version == remote_data_version {
					p.logger.Println(fmt.Sprintf(`restart ok: version: %v`,
						localClientdata.Version),
						time.Now().Format("2006-01-02 15:04:05"))
					b_success = true
					break
				}
				time.Sleep(time.Second)
			}
			if !b_success {
				p.logger.Println(
					time.Now().Format("2006-01-02 15:04:05"),
					fmt.Sprintf(`restart error:%v timeout.`,
						localClientdata.Version))
				time.Sleep(update_time * time.Second)
				continue
			} else {
				initSuccess := false
				for i := 0; i < 120; i++ {
					ret, err := InitClient(preinfo)
					if err != nil {
						time.Sleep(1e3)
						continue
					}
					if ret {
						initSuccess = true
						break
					}
					time.Sleep(1e3)
				}
				if !initSuccess {
					p.logger.Println(
						time.Now().Format("2006-01-02 15:04:05"),
						fmt.Sprintf(`init error:%v timeout.`,
							localClientdata.Version))
				}
			}
		}

		//		if __b_taskserver {
		//			if updateData.TaskServer.LastestVersion != taskServerClientdata.Version {
		//				p.do_update(updateData, localClientdata, false)
		//			}
		//		}
		time.Sleep(update_time * time.Second)
	}
}
func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

func main() {

	exec_dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	exec_dir = strings.Replace(exec_dir, `\`, `/`, -1)
	log_path := exec_dir + "/core_daemon.log"
	if runtime.GOOS != "windows" {
		log_path = "/var/log/fxqa/core_daemon.log"
	}
	logger := log.New(nil, "", 0)
	logger.SetOutput(&lumberjack.Logger{
		Filename:   log_path,
		MaxSize:    100, // megabytes
		MaxBackups: 3,
		MaxAge:     15, //days
	})

	_, err = os.Stat(log_path)
	if err != nil && os.IsNotExist(err) {
		//		log.Panic("Error: can not create logfile:", log_path)
	}

	serviceName := "FoxitQACoreDaemon"
	displayName := "Foxit QA Core Daemon"
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: displayName,
		Description: "Foxit QA Core Daemon Service.",
	}

	prg := &program{}
	prg.logger = logger

	s, err := service.New(prg, svcConfig)
	if err != nil {
		logger.Fatal(err)
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
	return
}
