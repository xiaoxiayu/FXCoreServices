package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/natefinch/lumberjack"

	"github.com/gorilla/websocket"
	cmap "github.com/streamrail/concurrent-map"
)

func (this *ManagerService) Init() error {
	var err error
	execlogpath := "/var/log/fxqa/testmanager.log"
	if runtime.GOOS == "windows" {
		execlogpath = "testmanager.log"
	} else {
		ret, _ := exists("/var/log/fxqa")
		if !ret {
			os.MkdirAll("/var/log/fxqa", 0777)
		}
	}

	this.logger = log.New(nil, "", log.Ldate|log.Ltime|log.Lshortfile)
	this.logger.SetOutput(&lumberjack.Logger{
		Filename:   execlogpath,
		MaxSize:    200, // megabytes
		MaxBackups: 3,
		MaxAge:     15, //days
	})

	this.logger.Println(`Log Start:`,
		time.Now().Format("2006-01-02 15:04:05"))
	_, err = os.Stat(execlogpath)
	if err != nil && os.IsNotExist(err) {
		log.Panic("Error: can not create logfile:", execlogpath)
	}

	//	if this.cfg.Discovery.Enabled {
	//		endpoints := []string{
	//			"http://" + this.cfg.Discovery.Server +
	//				":" + strconv.Itoa(this.cfg.Discovery.Port)}
	//		this.DisMaster = NewMaster(endpoints)
	//	}

	//this.UpdateTestServer()

	return nil
}

var upgrader = websocket.Upgrader{}

func (this *ManagerService) GetWorker(label string, taskConcurrent int, autowait bool) (idle_worker string, err error) {
	timeout := 3600
	t_i := 0

	if this.WSClientMap.Count() == 0 {
		err = fmt.Errorf("Worker not found.")
		return
	}
	haveLabel := false
	for _, tmpdata := range this.WSClientMap.Items() {
		machine_data := tmpdata.(*WSClient)
		machine_labels := strings.Split(machine_data.testdata.Label, ",")
		for _, mlabel := range machine_labels {
			if mlabel == label {
				haveLabel = true
				break
			}
		}
	}
	if !haveLabel {
		err = fmt.Errorf("Label not found.")
		return
	}

	for {
		for ip, tmpdata := range this.WSClientMap.Items() {

			data := tmpdata.(*WSClient)

			haveLabel := false
			machine_labels := strings.Split(data.testdata.Label, ",")
			for _, mlabel := range machine_labels {
				if mlabel == label {
					haveLabel = true
					break
				}
			}
			if !haveLabel {
				//err = fmt.Errorf("Label NoFound:%s", data.testdata.Label)
				continue
			}

			//			if data.testdata.Label != "" &&
			//				data.testdata.Label != label {
			//				//err = fmt.Errorf("Label Set Error")
			//				fmt.Println("*******")
			//				continue
			//			}
			//fmt.Println("GetCnt:", data.testdata.Ip, time.Now().Unix())
			if data.testdata.TaskCnt >= taskConcurrent {
				data.readcnt <- true
				select {
				case <-data.revcnt:
					if data.testdata.TaskCnt < taskConcurrent {
						idle_worker = ip
						return
					}
				case <-time.After(300 * time.Second):
					data.readcnt = make(chan bool, 1)
					data.revcnt = make(chan bool, 1)
					continue
				}
				time.Sleep(1e5)
			} else {
				idle_worker = ip
				return
			}

		}

		if t_i >= timeout {
			err = fmt.Errorf("GetIdleWoker Timeout")
			break
		}
	}

	return
}

func (this *ManagerService) Reset(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	label := r.FormValue("label")
	if label == "" {
		fmt.Fprintf(w, `{"ret":-1,"msg":"label not set."}`)
		return
	}
	this.TaskConcurrent.Set(label, 1)
	//this.TaskChan = make(chan retData, 1)
	fmt.Fprintf(w, `{"ret":0}`)
}

func (this *ManagerService) testStart(w http.ResponseWriter, r *http.Request) {
	//	fmt.Println("aa")
	r.ParseMultipartForm(32 << 20)

	res := `{"_type":"-1","_msg":"unknown error."}`
	defer func() {
		fmt.Fprintf(w, "%v", string(res))
	}()

	project := r.FormValue("project")
	if project == "" {
		project = "fx_compare"
	}

	running_flag := r.FormValue("running_flag")
	if running_flag == "" {
		running_flag = "-1"
	}

	forcekill_process := r.FormValue("forcekill_process")

	newurl := r.FormValue("newurl")
	oldurl := r.FormValue("oldurl")

	testfile_str := r.FormValue("testfile")
	test_str := r.FormValue("test")
	label_str := r.FormValue("label")
	oldver_str := r.FormValue("oldversion")
	newver_str := r.FormValue("newversion")

	timeout_str := GetFormValue(w, r, "timeout")
	autowait_str := GetFormValue(w, r, "autowait")
	bAutoWait := false
	if autowait_str != "" {
		bAutoWait = true
	}

	oldlib_str := GetFormValue(w, r, "oldlib")
	newlib_str := GetFormValue(w, r, "newlib")
	report_path_str := GetFormValue(w, r, "reportpath")
	custom_param_str := GetFormValue(w, r, "custom_param")
	//fmt.Println(r.Form)

	test_l := strings.Split(test_str, ".")
	if len(test_l) != 2 {
		res = `{"_type":"-1","_msg":"test name error"}`
		return
	}

	if this.WSClientMap.Count() == 0 {
		res = `{"_type":"-3","_msg":"Worker not found."}`
		return
	}

	taskcon_limit := 1
	_taskConn, bExists := this.TaskConcurrent.Get(label_str)
	if bExists {
		taskcon_limit = _taskConn.(int)
	}
	worker_ip, err := this.GetWorker(label_str, taskcon_limit, bAutoWait)
	if err != nil {
		res = fmt.Sprintf(`{"_type":"-3","_msg":"TestBusy.%s"}`, err.Error())
		return
	}
	if worker_ip == "" {
		res = `{"_type":"-4","_msg":"Test Server Busy."}`
		this.logger.Println("CannotGetWorkerIP")
		return
	}

	testcase_s := test_l[0]
	test_s := test_l[1]

	param_str := ""
	if project == "fx_compare" {
		param_str = fmt.Sprintf("--project=%s"+
			" --testfile=%s --test=%s --testcase=%s"+
			" --oldversion=%s --newversion=%s --oldlib=%s --newlib=%s"+
			" --statusmonitor=%s --timeout=%s --logserver=%s --reportpath=%s",
			"fx_compare",
			testfile_str,
			test_s,
			testcase_s,
			oldver_str,
			newver_str,
			oldlib_str,
			newlib_str,
			"127.0.0.1:10018",
			timeout_str,
			__logserver,
			report_path_str)
	} else if project == "fx_compare_v2" {
		param_str = fmt.Sprintf("--project=%s"+
			" --testfile=%s --test=%s --testcase=%s"+
			" --oldversion=%s --newversion=%s --oldlib=%s --newlib=%s"+
			" --statusmonitor=%s --timeout=%s"+
			" --logserver=%s --reportpath=%s --running_flag=%s"+
			" --newurl=%s --oldurl=%s",
			"fx_compare_v2",
			testfile_str,
			test_s,
			testcase_s,
			oldver_str,
			newver_str,
			oldlib_str,
			newlib_str,
			"127.0.0.1:10018",
			timeout_str,
			__logserver,
			report_path_str,
			running_flag,
			newurl,
			oldurl)

		if forcekill_process != "" {
			param_str += " --forcekill_process=" + forcekill_process
		}
	}

	if custom_param_str != "" {
		param_str += " --custom_param=" + custom_param_str
	}

	fmt.Println("WORKER IP:", worker_ip, time.Now().Unix())
	this.logger.Println("WORKER IP:", worker_ip, param_str)
	tmp, _ := this.WSClientMap.Get(worker_ip)
	g_c := tmp.(*WSClient)
	g_c.send <- []byte(param_str)
	fmt.Println("Prepare IP:", worker_ip)

	select {
	case ret_str := <-g_c.recive:
		res = string(ret_str)
	case <-time.After(300 * time.Second):
		g_c.send = make(chan []byte, 1256)
		g_c.readcnt = make(chan bool, 1)
		g_c.revcnt = make(chan bool, 1)
		g_c.recive = make(chan []byte, 1256)
		fmt.Println("TimeOut:", worker_ip)
		res = fmt.Sprintf(`{"_type":"-1","_msg":"%s:timeout"}`, worker_ip)
	}

}

type WSMessage struct {
	//	msgtype int
	msg string
}

type WSClient struct {
	client   *websocket.Conn
	msgqueue chan *WSMessage
	testdata TestData

	send    chan []byte
	recive  chan []byte
	readcnt chan bool
	revcnt  chan bool
	logger  *log.Logger
}

type TestRetData struct {
	TestCnt int `json:"testcnt"`
}

func (this *WSClient) ping(conMap cmap.ConcurrentMap) {
	ticker := time.NewTicker(3 * time.Second)
	defer func() {
		ticker.Stop()
		this.client.Close()
	}()
	for {
		select {
		case <-ticker.C:
			if err := this.client.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				if conMap.Has(this.testdata.Ip) {
					conMap.Remove(this.testdata.Ip)
				}
				fmt.Println("Ping ERROR:", this.testdata.Ip, err.Error())
				return

			} else {
				//fmt.Println("ping ok")
			}

		}
	}
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type RunParamData struct {
	Param []byte `json:"param"`
}

func (this *WSClient) run(conMap cmap.ConcurrentMap) {
	defer func() {
		this.client.Close()
	}()

	for {
		select {
		case message, _ := <-this.send:
			this.logger.Println("Run Start:", this.testdata.Ip, string(message))
			fmt.Println("Run Start:", this.testdata.Ip)
			var param RunParamData
			param.Param = message
			err := this.client.WriteJSON(param)
			if err != nil {
				this.logger.Printf("RunWriter Error:", this.testdata.Ip, err.Error())
				this.recive <- []byte(err.Error())
				return
			}

			err = this.client.ReadJSON(&this.testdata)
			if err != nil {
				this.logger.Printf("RunRead Error:", this.testdata.Ip, err.Error())
				this.recive <- []byte(err.Error())
				return
			} else {
				res, _ := json.Marshal(this.testdata)
				this.recive <- res
			}
			fmt.Println("Run End:", this.testdata.Ip)
			this.logger.Println("Run End:", this.testdata.Ip, string(message))

		case <-this.readcnt:
			//			this.logger.Println("Cnt Start:", this.testdata.Ip)
			var param RunParamData
			param.Param = []byte("cnt")
			err := this.client.WriteJSON(param)
			if err != nil {
				this.logger.Printf("CntWriter Error:", this.testdata.Ip, err.Error())
				this.revcnt <- true
				return
			}

			err = this.client.ReadJSON(&this.testdata)
			if err != nil {
				this.logger.Printf("CntRead Error:", this.testdata.Ip, err.Error())
				this.revcnt <- true
				return
			}
			this.revcnt <- true
			//			this.logger.Println("Cnt End:", this.testdata.Ip)

		}
	}
}

func (this *ManagerService) testStartKeepalive(w http.ResponseWriter, r *http.Request) {

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Print("upgrade:", err)
		return
	}
	//defer c.Close()
	//	this.WSClientMap.Set(strconv.Itoa(x), c)

	_, init_data, err := c.ReadMessage()
	if err != nil {
		this.logger.Printf("InitErr:%s", err.Error())
		fmt.Println("InitErr:%s", err.Error())
		return
	}
	var testdata TestData
	err = json.Unmarshal(init_data, &testdata)
	if err != nil {
		this.logger.Printf("ParseInitDataErr:%s", err.Error())
		fmt.Println("ParseInitDataErr:%s", err.Error(), string(init_data))
		return
	}

	for i := 0; i < 30; i++ {
		if this.WSClientMap.Has(string(testdata.Ip)) {
			this.logger.Printf("WS REREG WARN:%s", string(testdata.Ip))
			//return
			fmt.Println("WS REREG WARN:%s", string(testdata.Ip))
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}

	ws_client := &WSClient{client: c,
		msgqueue: make(chan *WSMessage, 1),
		testdata: testdata,
		logger:   this.logger,
		send:     make(chan []byte, 1256),
		readcnt:  make(chan bool, 1),
		revcnt:   make(chan bool, 1),
		recive:   make(chan []byte, 1256)}

	//	this.TaskCount.Set(string(testdata.Ip), testdata.TaskCnt)
	this.WSClientMap.Set(string(testdata.Ip), ws_client)
	fmt.Println("WS REG:", string(testdata.Ip), ",TaskCnt:", testdata.TaskCnt)

	go ws_client.ping(this.WSClientMap)
	ws_client.run(this.WSClientMap)

}

func (this *ManagerService) GetTestServer(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	label := r.FormValue("label")

	server_data := []TestData{}
	for _, tmpdata := range this.WSClientMap.Items() {
		data := tmpdata.(*WSClient)

		machine_labels := strings.Split(data.testdata.Label, ",")
		for _, mlabel := range machine_labels {
			if label == "" {
				server_data = append(server_data, (data.testdata))
				break
			} else if mlabel == label {
				server_data = append(server_data, (data.testdata))
			}
		}

	}

	data, err := json.Marshal(server_data)
	if err != nil {
		fmt.Fprintf(w, fmt.Sprintf(`{"ret":-2,"_msg":"%s"`, err.Error()))
		return
	}

	fmt.Fprintf(w, fmt.Sprintf(`{"ret":0,"data":%s}`, string(data)))
}

func (this *ManagerService) UpdateTestServer(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	label := r.FormValue("label")
	ip := r.FormValue("ip")
	if ip == "" {
		fmt.Fprintf(w, `{"ret":-1,"data":"ip is nil"}`)
		return
	}

	for _, tmpdata := range this.WSClientMap.Items() {
		data := tmpdata.(*WSClient)
		if data.testdata.Ip != ip {
			continue
		}
		data.testdata.Label = label
	}

	fmt.Fprintf(w, `{"ret":0}`)

}

//func (this *ManagerService) setTestVersion(w http.ResponseWriter, r *http.Request) {
//	r.ParseMultipartForm(32 << 20)

//	oldversion_str := GetFormValue(w, r, "old")
//	if oldversion_str == "" {
//		fmt.Fprintf(w, `{"_type":"-1","_msg":"old is null"`)
//		return
//	}
//	newversion_str := GetFormValue(w, r, "new")
//	if newversion_str == "" {
//		fmt.Fprintf(w, `{"_type":"-1","_msg":"new is null"`)
//		return
//	}
//	this.oldVersion = oldversion_str
//	this.newVersion = newversion_str
//}

//func (this *ManagerService) HeartBeat(w http.ResponseWriter, r *http.Request) {
//	r.ParseMultipartForm(32 << 20)
//	cdata := r.FormValue("data")
//	var data HeartbeatData
//	json.Unmarshal([]byte(cdata), &data)

//	data.TimeStamp = time.Now().Unix()
//	this.MachineInfo.Set(data.Ip, &data)
//	fmt.Fprintf(w, `{"ret":0}`)
//}

//func (this *ManagerService) TimerCleaner() {
//	var timeout_define time.Duration = 6
//	var ticker *time.Ticker = time.NewTicker(timeout_define * time.Second)
//	for t := range ticker.C {
//		//fmt.Println("Tick at", t.Unix())
//		for info := range this.MachineInfo.Items() {
//			//fmt.Println("Info:", info)
//			temdata, _ := this.MachineInfo.Get(info)
//			data := temdata.(*HeartbeatData)
//			if (t.Unix() - data.TimeStamp) > int64(timeout_define) {
//				this.MachineInfo.Remove(info)

//				tmp, _ := this.WSClientMap.Get(info)
//				//			g_c := tmp.(*websocket.Conn)
//				if tmp != nil {
//					g_c := tmp.(*WSClient)
//					g_c.client.Close()
//				}

//				this.WSClientMap.Remove(info)

//				this.TaskCount.Remove(info)
//				fmt.Println("Remove:", info)

//			}
//		}
//	}
//}

//func (this *ManagerService) TaskCountUpadate(w http.ResponseWriter, r *http.Request) {
//	r.ParseMultipartForm(32 << 20)
//	ip := r.FormValue("ip")
//	if ip == "" {
//		fmt.Fprintf(w, fmt.Sprintf(`{"ret":-1,"msg":"%s is not connected"}`, ip))
//		return
//	}

//	if !this.WSClientMap.Has(ip) {
//		fmt.Fprintf(w, `{"ret":-2,"msg":"ip not found"}`)
//		return
//	}
//	newcnt := 0
//	tmpdata, _ := this.WSClientMap.Get(ip)
//	data := tmpdata.(*WSClient)
//	if data.testdata.TaskCnt-1 > 0 {
//		newcnt = tmpcnt.(int) - 1
//	}
//	this.TaskCount.Set(ip, newcnt)
//	fmt.Fprintf(w, `{"ret":0}`)
//}

func (this *ManagerService) GetConcurrent(w http.ResponseWriter, r *http.Request) {
	label := r.FormValue("label")
	if label == "" {
		data := map[string]int{}
		for _l, _c := range this.TaskConcurrent.Items() {
			data[_l] = _c.(int)
		}
		jsdata, _ := json.Marshal(data)
		fmt.Fprintf(w, fmt.Sprintf(`{"ret":0,"data":%s}`, string(jsdata)))
		return
	}
	_taskCon, bExists := this.TaskConcurrent.Get(label)
	if !bExists {
		fmt.Fprintf(w, fmt.Sprintf(`{"ret":-2,"msg":"Label %s not set"}`, label))
	} else {
		fmt.Fprintf(w, fmt.Sprintf(`{"ret":0,"concurrent":%d}`, _taskCon.(int)))
	}

}

func (this *ManagerService) SetConcurrent(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	//	var err error
	label := r.FormValue("label")
	if label == "" {
		fmt.Fprintf(w, `{"ret":-1,"msg":"label is nil"}`)
		return
	}
	data_s := r.FormValue("set")
	if data_s == "" {
		fmt.Fprintf(w, `{"ret":-1,"msg":"set is nil"}`)
		return
	}
	data, err := strconv.Atoi(data_s)
	if err != nil {
		fmt.Fprintf(w, fmt.Sprintf(`{"ret":-2,"msg":"%s"}`, err.Error()))
		return
	}
	this.TaskConcurrent.Set(label, data)
	//	this.TaskChan = make(chan retData, data)
	//	this.TaskCountChan = make(chan retData, data)

	fmt.Fprintf(w, `{"ret":0}`)
}

func (this *ManagerService) Info(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	data := map[string]int{}
	for _l, _c := range this.TaskConcurrent.Items() {
		data[_l] = _c.(int)
	}
	jsdata, _ := json.Marshal(data)
	fmt.Fprintf(w, fmt.Sprintf(`{"ret":0,"version":"%s","concurrent":%s}`, __version, string(jsdata)))
}

func (this *ManagerService) testRunning() {

	//	for {
	//		select {
	//		case taskData := <-this.TaskChan:
	//			defer func() {
	//				if panic_err := recover(); panic_err != nil {
	//					taskData.retChan <- fmt.Sprintf(`{"_type":"-2","_msg":"Runtime Err:%s:%s"}`, taskData.taskIp, panic_err)
	//				}
	//			}()

	//			task_ip := taskData.taskIp
	//			ok := this.WSClientMap.Has(task_ip)
	//			if !ok {
	//				taskData.retChan <- fmt.Sprintf(`{"_type":"-2","_msg":"WSLink Err:%s"}`, task_ip)
	//				return
	//			}

	//			tmp, _ := this.WSClientMap.Get(task_ip)
	//			//			g_c := tmp.(*websocket.Conn)
	//			g_c := tmp.(*WSClient)

	//			g_c.msgqueue <- &WSMessage{msg: taskData.taskData}

	//			go func() {
	//				for msg := range g_c.msgqueue {

	//					//					g_c.client.SetWriteDeadline(time.Now().Add(writeWait))
	//					//					if !ok {
	//					//						// The hub closed the channel.
	//					//						g_c.client.WriteMessage(websocket.CloseMessage, []byte{})
	//					//						return
	//					//					}
	//					//					fmt.Println(string(msg.msg))

	//					//					w, err := g_c.client.NextWriter(websocket.TextMessage)
	//					//					if err != nil {
	//					//						return
	//					//					}
	//					//					w.Write([]byte(msg.msg))

	//					err := g_c.client.WriteMessage(1, []byte(msg.msg))
	//					if err != nil {
	//						this.logger.Printf("WErr:%s-%s", err.Error(), task_ip)
	//						g_c.client.Close()
	//						this.WSClientMap.Remove(task_ip)
	//						fmt.Println("WS DEL0:", task_ip, err.Error())
	//						taskData.retChan <- fmt.Sprintf(`{"_type":"-2","_msg":"%s:%s"}`, task_ip, err.Error())
	//						return
	//					}
	//				}

	//			}()
	//			_, ret_str, err := g_c.client.ReadMessage()
	//			if err != nil {
	//				this.logger.Printf("RErr:%s-%s", err.Error(), task_ip)
	//				g_c.client.Close()
	//				this.WSClientMap.Remove(task_ip)
	//				fmt.Println("WS DEL1:", task_ip, err.Error())
	//				taskData.retChan <- fmt.Sprintf(`{"_type":"-2","_msg":"%s:%s"}`, task_ip, err.Error())
	//				return
	//			} else {
	//				var client_data TestData
	//				err := json.Unmarshal(ret_str, &client_data)
	//				if err == nil {
	//					this.TaskCount.Set(task_ip, client_data.TaskCnt)
	//					fmt.Println("START:", client_data.TaskCnt)
	//					// Wait Test End.
	//					//					go func(ip string) {
	//					//						_, ret_str, err := g_c.client.ReadMessage()
	//					//						if err != nil {
	//					//							fmt.Println("ERROR:ReadEND:", err.Error())
	//					//						}
	//					//						var runend_data TestData
	//					//						json.Unmarshal(ret_str, &runend_data)
	//					//						this.TaskCount.Set(ip, runend_data.TaskCnt)
	//					//						fmt.Println("END:", runend_data.TaskCnt)

	//					//					}(task_ip)

	//				}
	//			}

	//			taskData.retChan <- string(ret_str)
	//			//		case <-ticker.C:
	//			//			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	//			//			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
	//			//				return
	//			//			}

	//		case <-time.After(1e7):
	//			continue

	//			//		case <-time.After(pingPeriod):
	//			//			continue

	//		}
	//	}
}
