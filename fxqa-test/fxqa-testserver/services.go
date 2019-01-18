package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"syscall"

	"errors"

	"encoding/json"
	"io/ioutil"
	"os/exec"

	"github.com/inconshreveable/go-update"
	"github.com/kardianos/osext"

	"net/url"
	"strconv"
	"strings"
	"time"

	//fxqacommon "foxitsoftware.cn/quality_control/services/fxqa-tool/common"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/matishsiao/goInfo"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

func (this *TestService) Init() {
	//	this.logfile = make(map[string]*os.File)
	//	this.logger = make(map[string]*log.Logger)

	var err error

	execlogpath := "/var/log/fxqa/test.log"
	if runtime.GOOS == "windows" {
		execlogpath = "test.log"
	} else {
		ret, _ := exists("/var/log/fxqa")
		if !ret {
			os.MkdirAll("/var/log/fxqa", 0777)
		}
	}

	this.logfile, err = os.OpenFile(execlogpath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0)
	if err != nil {
		fmt.Printf("%s\r\n", err.Error())
		os.Exit(-1)
	}
	this.logger = log.New(this.logfile, "\r\n", log.Ldate|log.Ltime|log.Llongfile)

	logpath := "/var/log/fxqa/test.log"
	if runtime.GOOS == "windows" {
		logpath = "test.log"
	}

	this.logfile, err = os.OpenFile(logpath, os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		fmt.Printf("%s\r\n", err.Error())
		os.Exit(-1)
	}
	this.logger = log.New(this.logfile, "\r\n", log.Ldate|log.Ltime|log.Llongfile)

}

func (this *TestService) Log(test_key, logstr string) {
	if len(logstr) > 0 {
		this.logger.Println(logstr)
	}
}

func (this *TestService) RemoteLog(test_type, logserver, data string) error {
	if logserver == "" {
		return nil
	}

	logdata := make(url.Values)
	logdata["s"] = []string{string(data)}
	logdata["n"] = []string{string(test_type)}
	_, err := HTTPPost(logserver, logdata)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	//	fmt.Println(string(logserver))
	//	fmt.Println(string(res))
	//	fmt.Println(logdata)
	return nil

}

func (this *TestService) Update(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	new_package_url := r.FormValue("url")

	resp, err := http.Get(new_package_url)
	if err != nil {

		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"err":"download error"}`))
		return
	}
	defer resp.Body.Close()
	err = update.Apply(resp.Body, update.Options{})
	if err != nil {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"err":"update apply error"}`))
	}

	fmt.Fprintf(w, "%v", `{"ret":0}`)
}

func (this *TestService) fxcoreLoop() {
	timeout_limit := 100
	for {
		if __task_server == "" || __task_ws_path == "" {
			time.Sleep(1 * time.Second)
			continue
		}
		if this.LinkRetryIndex >= timeout_limit {
			time.Sleep(1 * time.Second)
			continue
		}

		err := this.wsLink(__task_server, __task_ws_path)
		errChan := make(chan error)
		if err == nil {
			this.LinkRetryIndex = 0
			this.logger.Println(fmt.Sprintf("Connect %s OK", __task_server))
			__test_is_online = true
			go this.fxcoreTestKL(errChan)
		} else {
			__test_is_online = false
			this.LinkRetryIndex++
			this.logger.Println(fmt.Sprintf("Network ERROR: %s", err.Error()))
			time.Sleep(10e9)
			continue
		}

		select {
		case errParam := <-errChan:
			this.logger.Println(fmt.Sprintf("WSERR:%s", errParam.Error()))
			return
		}
	}
}

func (this *TestService) fxcoreLKStart() {
	for {
		if __task_server == "" || __task_ws_path == "" {
			time.Sleep(1 * time.Second)
			continue
		}
		this.fxcoreLoop()
	}
}

func (this *TestService) wsLink(ws_host, ws_path string) (err error) {
	u := url.URL{Scheme: "ws", Host: ws_host, Path: ws_path}
	//	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		//this.Log("fxcore", fmt.Sprintf("WSLINKERR:%s", err.Error()))
		fmt.Println("fxcore", fmt.Sprintf("WSLINKERR:%s%s, %s",
			ws_host, ws_path, err.Error()))
		return
	}
	this.wsconn = c
	return
}

type TestData struct {
	Ip      string `json:"ip"`
	TaskCnt int    `json:"taskcount"`
	Label   string `json:label`
}

type RunParamData struct {
	Param []byte `json:"param"`
}

func getFXCompareParamValue(key, param_str string) string {
	tem_list := strings.Split(param_str, "--"+key+"=")
	if len(tem_list) <= 1 {
		return ""
	}
	value_str := tem_list[1]

	if strings.Index(value_str, "--") == -1 {
		return value_str
	}
	value_str = value_str[0:strings.Index(value_str, "--")]
	value_str = strings.TrimSpace(value_str)

	return value_str
}

func (this *TestService) fxcoreTestKL(err_chan chan error) {

	defer this.wsconn.Close()

	machine_data, _ := json.Marshal(TestData{Ip: G_LOCALIP, TaskCnt: 0, Label: __machine_label})
	err := this.wsconn.WriteMessage(websocket.TextMessage, machine_data)
	if err != nil {
		this.wsconn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		err_chan <- fmt.Errorf("WSERR WriteIP:%s", err.Error())
		return
	}
	//this.wsconn.SetReadDeadline(time.Now().Add(30 * time.Second))
	for {
		var param RunParamData
		err := this.wsconn.ReadJSON(&param)
		if err != nil {
			fmt.Println("Error:", err.Error())
			err_chan <- fmt.Errorf("WSERR Read:%s", err.Error())
			return
		}
		param_str := string(param.Param)

		if param_str == "cnt" {
			//If finish. update taskcnt.
			this.TaskCntLocker.RLock()
			testdata := TestData{
				Ip:      G_LOCALIP,
				TaskCnt: __machine_task_cnt,
				Label:   __machine_label}
			this.TaskCntLocker.RUnlock()
			//			this.logger.Println("CntEnd:", param_str)
			err = this.wsconn.WriteJSON(testdata)
			if err != nil {
				err_chan <- fmt.Errorf("WSERR WriteID:%s", err.Error())
				return
			}
			continue
		}

		fmt.Println(param_str)

		test_id := RandomAlphaOrNumeric(5, true, true)
		test_id = strconv.FormatInt(time.Now().Unix(), 10) + test_id

		testfile_str := getFXCompareParamValue("testfile", param_str)
		testcase_str := getFXCompareParamValue("testcase", param_str)
		test_str := getFXCompareParamValue("test", param_str)
		timeout_str := getFXCompareParamValue("timeout", param_str)
		project_str := getFXCompareParamValue("project", param_str)
		logserver_str := getFXCompareParamValue("logserver", param_str)
		oldversion_str := getFXCompareParamValue("oldversion", param_str)
		newversion_str := getFXCompareParamValue("newversion", param_str)
		custom_param_str := getFXCompareParamValue("custom_param", param_str)
		forcekill_process_str := getFXCompareParamValue("forcekill_process", param_str)

		this.TaskCntLocker.Lock()
		__machine_task_cnt++
		this.TaskCntLocker.Unlock()

		go func(project, param, testid, testcase, test, testfile, timeout,
			logserver, oldversion, newversion, forcekill_process, custom_param string) {
			this.logger.Println("RunStart:", string(param))
			logstr, err := Run("fxcore", testfile,
				__program_path, param, timeout, forcekill_process, custom_param)
			this.TaskCntLocker.Lock()
			__machine_task_cnt--
			this.TaskCntLocker.Unlock()
			if err != nil {
				this.Log("fxcore",
					fmt.Sprintf(`{"_type":"-1","_msg":"%s",`+
						`"testcase":"%s","test":"%s","testfile":"%s"}`,
						err.Error(), testcase, test, testfile))
				err_type := "Crash"
				if err.Error() == "Death" {
					err_type = "Death"
				}
				js_testfile, _ := json.Marshal(testfile)
				errlog_str := fmt.Sprintf(
					`{"production":"%s","server":"%s","pid":"-1","version":"%s",`+
						`"testcase":"%s","test":"%s","testfile":%s,"type":"%s"}`,
					project, G_LOCALIP, oldversion+"*"+newversion,
					testcase, test, string(js_testfile), err_type)
				this.RemoteLog("fxcore", "http://"+logserver+"/logs", errlog_str)
			} else {
				this.logger.Println("RunResult:", string(logstr))
			}

		}(project_str,
			param_str,
			test_id,
			testcase_str,
			test_str,
			testfile_str,
			timeout_str,
			logserver_str,
			oldversion_str,
			newversion_str,
			forcekill_process_str,
			custom_param_str)

		this.TaskCntLocker.RLock()
		testdata := TestData{
			Ip:      G_LOCALIP,
			TaskCnt: __machine_task_cnt,
			Label:   __machine_label}
		this.TaskCntLocker.RUnlock()
		this.logger.Println("RunTriEnd:", param_str)
		err = this.wsconn.WriteJSON(testdata)
		if err != nil {
			err_chan <- fmt.Errorf("WSERR WriteID:%s", err.Error())
			return
		}

	}

}

func (this *TestService) InitParam(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	program_path := r.FormValue("program")
	if program_path == "" {
		fmt.Fprintf(w, "%v", `{"ret":-1,"msg":"program is nil"}`)
		return
	}

	taskserver := r.FormValue("taskserver")
	if taskserver == "" {
		fmt.Fprintf(w, "%v", `{"ret":-1,"msg":"taskserver is nil"}`)
		return
	}

	hearbeat_url := r.FormValue("heartbeaturl")
	if hearbeat_url == "" {
		hearbeat_url = "http://10.103.129.79/test/state/heartbeat"
	}

	localip := r.FormValue("localip")
	if localip != "" {
		G_LOCALIP = localip
	}

	label := r.FormValue("label")

	__task_server = taskserver
	__machine_label = label
	__task_server = taskserver
	__program_path = program_path
	__heartbeat_url = hearbeat_url

	data, err := json.Marshal(CfgData{
		Ip:           G_LOCALIP,
		TaskServer:   __task_server,
		Label:        __machine_label,
		ProgramPath:  __program_path,
		HeartBeatUrl: __heartbeat_url})
	if err != nil {
		fmt.Fprintf(w, "%v", `{"ret":-2,"msg":"cfg write failed"}`)
		return
	}
	err = ioutil.WriteFile(__cfgpath, data, 0644)
	if err != nil {
		fmt.Fprintf(w, "%v", `{"ret":-2,"msg":"cfg write failed"}`)
		return
	}

	fmt.Fprintf(w, "%v", `{"ret":0}`)
}

func (this *TestService) Reset(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	__task_server = ""
	__task_server = ""
	__task_ws_path = ""
	__program_path = ""

	fmt.Fprintf(w, "%v", `{"ret":0}`)
}

func (this *TestService) Info(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	filename, _ := osext.Executable()
	json_filename, _ := json.Marshal(filename)

	fmt.Fprintf(w, "%v",
		fmt.Sprintf(`{"ret":0,"running_count":%d,"version":%s,"taskserver":"%s","program":"%s","label":"%s","exec":%s}`,
			__machine_task_cnt, __version, __task_server, __program_path, __machine_label, json_filename))
}

func (this *TestService) GetLabel(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"label":"%s"}`, __machine_label))
}

func (this *TestService) SetLabel(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	adddata := r.FormValue("add")

	if adddata != "" {
		if __machine_label == "" {
			__machine_label = adddata
		} else {
			__machine_label += "," + adddata
		}
	}
	setdata := r.FormValue("set")
	if setdata != "" {
		if setdata == __machine_label {
			fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"label":"%s"}`, __machine_label))
			return
		}
		__machine_label = setdata
	}

	putdata := make(url.Values)
	putdata["ip"] = []string{G_LOCALIP}
	putdata["label"] = []string{__machine_label}

	_, err := HTTPPut("http://"+__task_server+"/testserver", putdata)
	if err != nil {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"msg":"%s"}`, err.Error()))
		return
	}

	data, err := json.Marshal(CfgData{
		Ip:           G_LOCALIP,
		TaskServer:   __task_server,
		Label:        __machine_label,
		ProgramPath:  __program_path,
		HeartBeatUrl: __heartbeat_url})
	if err != nil {
		fmt.Fprintf(w, "%v", `{"ret":-2,"msg":"cfg write failed"}`)
		return
	}
	err = ioutil.WriteFile(__cfgpath, data, 0644)
	if err != nil {
		fmt.Fprintf(w, "%v", `{"ret":-2,"msg":"cfg write failed"}`)
		return
	}

	fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"label":"%s"}`, __machine_label))
}

func (this *TestService) DelLabel(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	__machine_label = ""

	putdata := make(url.Values)
	putdata["ip"] = []string{G_LOCALIP}
	putdata["label"] = []string{""}

	_, err := HTTPPut("http://"+__task_server+"/testserver", putdata)
	if err != nil {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"msg":"%s"}`, err.Error()))
		return
	}

	data, err := json.Marshal(CfgData{
		Ip:           G_LOCALIP,
		TaskServer:   __task_server,
		Label:        __machine_label,
		ProgramPath:  __program_path,
		HeartBeatUrl: __heartbeat_url})
	if err != nil {
		fmt.Fprintf(w, "%v", `{"ret":-2,"msg":"cfg write failed"}`)
		return
	}
	err = ioutil.WriteFile(__cfgpath, data, 0644)
	if err != nil {
		fmt.Fprintf(w, "%v", `{"ret":-2,"msg":"cfg write failed"}`)
		return
	}

	fmt.Fprintf(w, "%v", `{"ret":0,"label":""}`)
}

func (this *TestService) fxcoreTest(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	ret_msg := ""
	ret_type := "0"
	test_id := ""
	param_str := ""
	defer func() {
		fmt.Fprintf(w, "%v",
			fmt.Sprintf(`{"_type":"%s","_msg":"%s","host":"%s","testid":"%s"}`,
				ret_type, ret_msg, G_LOCALIP, test_id))
		this.logger.Println(param_str)
	}()

	testcase_str := GetFormValue(w, r, "testcase")
	test_str := GetFormValue(w, r, "test")
	logserver_str := GetFormValue(w, r, "logserver")

	testfile_str := GetFormValue(w, r, "testfile")
	testfile_str = strings.Replace(testfile_str, "/mnt/mfs/", "/fxdata/", 1)

	async := GetFormValue(w, r, "async")
	if async == "" {
		async = "0"
	}

	project_str := GetFormValue(w, r, "project")
	//fmt.Println("ProjectStr:", project_str)
	if project_str == "fx_compare" {
		oldversion_str := GetFormValue(w, r, "oldversion")
		newversion_str := GetFormValue(w, r, "newversion")
		oldlib_str := GetFormValue(w, r, "oldlib")
		newlib_str := GetFormValue(w, r, "newlib")
		statusmonitor_str := GetFormValue(w, r, "statusmonitor")
		timeout_str := GetFormValue(w, r, "timeout")
		report_path_str := GetFormValue(w, r, "reportpath")

		param_str = fmt.Sprintf("--project=%s"+
			" --testfile=%s --test=%s --testcase=%s"+
			" --oldversion=%s --newversion=%s --oldlib=%s --newlib=%s"+
			" --statusmonitor=%s --timeout=%s --logserver=%s --reportpath=%s",
			"fx_compare",
			testfile_str,
			test_str,
			testcase_str,
			oldversion_str,
			newversion_str,
			oldlib_str,
			newlib_str,
			statusmonitor_str,
			timeout_str,
			logserver_str,
			report_path_str)
	}

	//fmt.Println(param_str)
	test_id = RandomAlphaOrNumeric(5, true, true)
	test_id = strconv.FormatInt(time.Now().Unix(), 10) + test_id
	if async == "0" {
		func(param string) {
			//			logstr, err := Run("rdk_unittest",
			//				test_id, testfile_str,
			//				this.cfg.Test["rdk_unittest"].Program,
			//				param, this.cacheServer["fxcore"])
			//			if err != nil {
			//				this.Log("rdk_unittest",
			//					fmt.Sprintf(`{"_type":"-1","_msg":"%s",`+
			//						`"testcase":"%s","test":"%s","testfile":"%s"}`,
			//						testcase_str, test_str, testfile_str, err.Error()))
			//				ret_type = "-1"
			//				ret_msg = err.Error()
			//			} else {
			//				this.Log("rdk_unittest", string(logstr))
			//			}
		}(param_str)
	} else {
		go func(param string) {
			//			logstr, err := Run("rdk_unittest",
			//				test_id, testfile_str,
			//				this.cfg.Test["rdk_unittest"].Program, param,
			//				this.cacheServer["rdk_unittest"])
			//			if err != nil {
			//				this.Log("rdk_unittest",
			//					fmt.Sprintf(`{"_type":"-1","_msg":"%s",`+
			//						`"testcase":"%s","test":"%s","testfile":"%s"}`,
			//						testcase_str, test_str, testfile_str, err.Error()))
			//			} else {
			//				this.Log("rdk_unittest", string(logstr))
			//			}

		}(param_str)
	}

}

func (this *TestService) Upload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	this.logger.Println(fmt.Sprintf(`Upload: %v`, r.Form),
		time.Now().Format("2006-01-02 15:04:05"))

	savepath := r.FormValue("savepath")
	file, _, err := r.FormFile("file")
	if err != nil {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"err":"%s"}`, err.Error()))
		return
	}

	//fmt.Println(savepath)
	//	checkErr(err)
	f, err := os.OpenFile(savepath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"err":"%s"}`, err.Error()))
		return
	}
	io.Copy(f, file)
	//	checkErr(err)
	defer f.Close()
	defer file.Close()
	fmt.Fprintf(w, "%v", `{"ret":0}`)

}

func (this *TestService) Kill(w http.ResponseWriter, r *http.Request) {
	ret_type := 0
	ret_msg := ""
	var pids []string
	var info string
	defer func() {
		ret_str := fmt.Sprintf(`{"ret":"%d","_msg":"%s"}`,
			ret_type, ret_msg)
		fmt.Fprintf(w, "%v", ret_str)
		this.logger.Println(info + ":" + ret_str)
	}()

	r.ParseMultipartForm(32 << 20)

	type_s := r.FormValue("type")
	if "pid" == type_s {
		//		if runtime.GOOS == "windows" {
		//			ret_msg = "windows unsupport"
		//			return
		//		}

		pids = r.Form["id"]
		if len(pids) == 0 {
			ret_type = -1
			ret_msg = "pid is null"
			return
		}
		infos := r.Form["info"]
		if len(infos) > 0 {
			info = infos[0]
		}

		for _, pid_s := range pids {
			err := Kill(pid_s, syscall.Signal(0x9)) // syscall.SIGKILL
			if err != nil {
				ret_type = -1
				ret_msg += fmt.Sprintf(`pid-%s:%s`, pid_s, err.Error())
			}
		}
	} else if "name" == type_s {
		var names []string
		names = r.Form["name"]
		if len(names) == 0 {
			ret_type = -1
			ret_msg = "name is null"
			return
		}

		for _, name_s := range names {
			err := KillByName(name_s)
			if err != nil {
				ret_type = -1
				ret_msg += fmt.Sprintf(`%s:%s`, name_s, err.Error())
			}
		}
	} else {
		ret_msg = "type should be pid or name"
	}
}

func parseEditorCommand(editorCmd string) (string, []string, error) {
	var args []string
	state := "start"
	current := ""
	quote := "\""
	for i := 0; i < len(editorCmd); i++ {
		c := editorCmd[i]

		if state == "quotes" {
			if string(c) != quote {
				current += string(c)
			} else {
				args = append(args, current)
				current = ""
				state = "start"
			}
			continue
		}

		if c == '"' || c == '\'' {
			state = "quotes"
			quote = string(c)
			continue
		}

		if state == "arg" {
			if c == ' ' || c == '\t' {
				args = append(args, current)
				current = ""
				state = "start"
			} else {
				current += string(c)
			}
			continue
		}

		if c != ' ' && c != '\t' {
			state = "arg"
			current += string(c)
		}
	}

	if state == "quotes" {
		return "", []string{}, errors.New(fmt.Sprintf("Unclosed quote in command line: %s", editorCmd))
	}

	if current != "" {
		args = append(args, current)
	}

	if len(args) <= 0 {
		return "", []string{}, errors.New("Empty command line")
	}

	if len(args) == 1 {
		return args[0], []string{}, nil
	}

	return args[0], args[1:], nil
}

func (this *TestService) CmdRun(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	cmd_s := r.FormValue("cmd")

	//	var cmd_str string
	var err error

	cmd_list := []string{}
	if runtime.GOOS == "windows" {
		cmd_list = append(cmd_list, "cmd.exe")
		cmd_list = append(cmd_list, "/c")
		cmd_list = append(cmd_list, "call")
	}

	exe_path, exe_param, err := parseEditorCommand(cmd_s)
	if err != nil {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":2,"errlog":"%s"}`, "cmd str parse error"))
		return
	}
	cmd_list = append(cmd_list, string(exe_path))
	cmd_list = append(cmd_list, exe_param...)

	this.logger.Println(
		time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(`cmdrun : %v`, cmd_list))

	err_log := ""
	log_str := []byte{}
	defer func() {
		if len(err_log) > 0 {
			_data, _ := json.Marshal(err_log)
			fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":2,"errlog":%s}`, string(_data)))
		} else {
			_data, _ := json.Marshal(string(log_str))
			fmt.Println(string(log_str))
			fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"log":%s}`, string(_data)))
		}
		this.logger.Println(
			time.Now().Format("2006-01-02 15:04:05"),
			fmt.Sprintf(`cmdrun end: %v`, cmd_list))

	}()

	cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
	//	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdout, exec_err := cmd.StdoutPipe()
	if err != nil {
		this.logger.Println(
			time.Now().Format("2006-01-02 15:04:05"),
			fmt.Sprintf(`cmdrun error: %v`, cmd_list))
		err = exec_err
		return
	}

	exec_err = cmd.Start()
	if exec_err != nil {
		err_log = exec_err.Error()
		this.logger.Println(
			time.Now().Format("2006-01-02 15:04:05"),
			fmt.Sprintf(`cmdrun start error: %v`, err_log))
		return
	}

	log_str, exec_err = ioutil.ReadAll(stdout)
	if exec_err != nil {
		err_log = exec_err.Error()
		this.logger.Println(
			time.Now().Format("2006-01-02 15:04:05"),
			fmt.Sprintf(`cmdrun read errro: %v`, err_log))
		return
	}

	exec_err = cmd.Wait()
	if exec_err != nil {
		err_log = exec_err.Error()
		err = exec_err
		this.logger.Println(
			time.Now().Format("2006-01-02 15:04:05"),
			fmt.Sprintf(`cmdrun wait errro: %v`, err_log))
		return
	}
}

func (this *TestService) ClearLog(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	logpath := "/var/log/fxqa/test.log"
	if runtime.GOOS == "windows" {
		logpath = "test.log"
	}
	_, err := exec.Command(
		"bash",
		"-c",
		fmt.Sprintf(`echo "" > %s`, logpath)).Output()
	if err != nil {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"_msg":"%s"}`, err.Error()))
	}

	fmt.Fprintf(w, "%v", `{"ret":0}`)
}

func (this *TestService) LogDownload(w http.ResponseWriter, r *http.Request) {

}

func (this *TestService) GetOnline(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"online":%v}`, __test_is_online))
}

func (this *TestService) SetOnline(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	if __test_is_online {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"online":%v}`, __test_is_online))
		return
	}
	this.LinkRetryIndex = 0
	fmt.Fprintf(w, "%v", `{"ret":0,"msg":"linking"}`)
}

func getMemoryInfo() (*mem.VirtualMemoryStat, error) {
	v, err := mem.VirtualMemory()

	// almost every return value is a struct
	//info := fmt.Sprintf("Total: %v MB, Free:%v, UsedPercent:%f%%\n", v.Total/1024/1024, v.Free, v.UsedPercent)

	// convert to JSON. String() is also implemented
	return v, err
}

func getCpuInfo(interval time.Duration) ([]float64, error) {
	//	cpuinfo, err := cpu.Info()
	//	if err != nil {
	//		return
	//	}
	//	_msg, err := json.Marshal(cpuinfo)
	//	if err != nil {
	//		return
	//	}
	//	var ret_msg string
	//	if len(ret_msg) > 0 {
	//		ret_msg += ","
	//	}
	//	ret_msg += `"cpu_info":` + string(_msg)
	//	fmt.Println(ret_msg)
	percpu := true
	data, err := cpu.Percent(interval, percpu)

	if err != nil {
		return nil, err
	}
	return data, err
}

func getDiskInfo() ([]*disk.UsageStat, error) {

	partitions, err := disk.Partitions(true)
	if err != nil {
		return nil, err
	}
	var datas []*disk.UsageStat
	for _, partition := range partitions {
		//		fmt.Println(partition)
		data, err := disk.Usage(partition.Device)
		if err != nil {
			continue
		}
		datas = append(datas, data)
	}
	return datas, nil
}

type hardwareUseageData struct {
	CPU      []float64              `json:"cpu"`
	Mem      *mem.VirtualMemoryStat `json:"memory"`
	Disk     []*disk.UsageStat      `json:"disk"`
	OS       string                 `json:"os"`
	Hostname string                 `json:"hostname"`
	Platform string                 `json:"platform"`
	CPUs     int                    `json:"cpus"`
}

func (this *TestService) GetHardwareInfo(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	var mem *mem.VirtualMemoryStat
	var cpu []float64
	var disk []*disk.UsageStat

	type_s := r.FormValue("type")
	time_s := r.FormValue("time")
	var time_i int
	if time_s == "" {
		time_i = 1
	}

	if type_s == "memory" {
		mem, _ = getMemoryInfo()
	} else if type_s == "cpu" {
		cpu, _ = getCpuInfo(time.Second * (time.Duration)(time_i))

	} else if type_s == "disk" {
		disk, _ = getDiskInfo()
	} else {
		disk, _ = getDiskInfo()
		cpu, _ = getCpuInfo(time.Second * (time.Duration)(time_i))
		mem, _ = getMemoryInfo()
	}

	gi := goInfo.GetInfo()
	var data hardwareUseageData
	data.CPU = cpu
	data.Disk = disk
	data.Mem = mem
	data.OS = gi.OS
	data.Platform = gi.Platform
	data.CPUs = gi.CPUs
	data.Hostname = gi.Hostname
	js_data, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"data":%s}`, js_data))
}

type HeartbeatData struct {
	Name        string `json:"machine"`
	Status      string `json:"status"`   // free or busy
	TestInfo    string `json:"testinfo"` // running test
	MachineInfo string `json:"machineinfo"`
}

func HeartBeatStart(id, ip string, interval time.Duration) {
	var hbdata HeartbeatData
	hbdata.MachineInfo = __machine_info

	hbdata.Name = fmt.Sprintf("%s_%s_%d", id, ip, os.Getpid())
	//	fmt.Println(hbdata.Name)
	for true {
		hbdata.Status = ""
		tmp, bexist := __heartbeat_info.Get("status")
		if bexist {
			hbdata.Status = tmp.(string)
		}
		tmp, bexist = __heartbeat_info.Get("testinfo")
		if bexist {
			hbdata.TestInfo = tmp.(string)
		}
		//hbdata.TimeStamp = time.Now().UnixNano() / 1000000
		jsinfo, _ := json.Marshal(hbdata)
		data := make(url.Values)
		data["data"] = []string{string(jsinfo)}
		//		fmt.Println(data)

		HTTPPost(__heartbeat_url, data)
		//		fmt.Println(__heartbeat_url)
		time.Sleep(interval * time.Second)
	}

}

func (this *TestService) GetFuzzData(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	type_s := r.FormValue("type")
	name_s := r.FormValue("name")
	if name_s == "" && type_s == "" {
		all_data, _ := json.Marshal(this.fuzzoutput_folder.Keys())
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"data":%s}`, string(all_data)))
		return
	}
	if name_s == "" {
		fmt.Fprintf(w, "%v", `{"ret":-1,"err":"name is nil"}`)
		return
	}
	_data_path, _ret := this.fuzzoutput_folder.Get(name_s)
	if !_ret {
		fmt.Fprintf(w, "%v", `{"ret":-1,"err":"data folder path not set"}`)
		return
	}
	data_path := _data_path.(string)
	if type_s == "bitmap" {
		data, err := ioutil.ReadFile(fmt.Sprintf("%s/fuzz_bitmap", data_path))
		if err != nil {
			fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"err":"%s"}`, string(data)))
			return
		}
		//		base64.StdEncoding.EncodeToString(data)
		jsdata, _ := json.Marshal(data)
		//		fmt.Println(jsdata)
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"data":%v}`, string(jsdata)))
		return
	} else if type_s == "plot" {
		//		fmt.Println(fmt.Sprintf("%s/plot_data", data_path))
		data, err := ioutil.ReadFile(fmt.Sprintf("%s/plot_data", data_path))
		if err != nil {
			fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"err":"%s"}`, string(data)))
			return
		}
		jsdata, _ := json.Marshal(data)
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"data":%s}`, string(jsdata)))
		return
	} else if type_s == "stats" {
		data, err := ioutil.ReadFile(fmt.Sprintf("%s/fuzzer_stats", data_path))
		if err != nil {
			fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"err":"%s"}`, string(data)))
			return
		}
		jsdata, _ := json.Marshal(data)
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":0,"data":%s}`, string(jsdata)))
		return
	}
	fmt.Fprintf(w, "%v", `0`)
}

func (this *TestService) SetFuzzData(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	name_s := r.FormValue("name")
	path_s := r.FormValue("path")
	if name_s == "" {
		fmt.Fprintf(w, "%v", `{"ret":-1,"err":"name is nil"}`)
		return
	}
	if path_s == "" {
		fmt.Fprintf(w, "%v", `{"ret":-1,"err":"path is nil"}`)
		return
	}
	this.fuzzoutput_folder.Set(name_s, path_s)
	fmt.Fprintf(w, "%v", `{"ret":0}`)
}

func (this *TestService) DelFuzzData(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	vars := mux.Vars(r)
	key := vars["name"]

	this.fuzzoutput_folder.Remove(key)
	fmt.Fprintf(w, "%v", `{"ret":0}`)
}
