package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	//	"syscall"
	"time"
)

func KillByName(process_name string) error {
	var cmd_str string
	if runtime.GOOS == "windows" {
		cmd_str = fmt.Sprintf("taskkill /f /im %s", process_name)

	} else {
		cmd_str = fmt.Sprintf("pkill %s", process_name)

	}
	cmd_list := strings.Split(cmd_str, " ")

	cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func GetFormValue(w http.ResponseWriter, r *http.Request, form_name string) string {
	defer func() string {
		if err := recover(); err != nil {
			//fmt.Println(err)
			//fmt.Fprintf(w, "%s", err)
			return ""
		}
		return r.Form[form_name][0]
	}()
	return r.Form[form_name][0]
}

func GetFormValues(w http.ResponseWriter, r *http.Request, form_name string) []string {
	defer func() []string {
		if err := recover(); err != nil {
			//fmt.Println(err)
			//fmt.Fprintf(w, "%s", err)
			return nil
		}
		return r.Form[form_name]
	}()
	return r.Form[form_name]
}

func HTTPPost(url_str string, data url.Values) (string, error) {
	res, err := http.PostForm(url_str, data)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func HTTPPut(url_str string, data url.Values) (string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("PUT", url_str, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func HTTPGet(url_str string) ([]byte, error) {
	client := &http.Client{}

	reqest, err := http.NewRequest("GET", url_str, nil)
	if err != nil {
		return nil, err
	}

	reqest.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	reqest.Header.Add("Accept-Language", "ja,zh-CN;q=0.8,zh;q=0.6")
	reqest.Header.Add("Connection", "keep-alive")
	reqest.Header.Add("Cookie", "设置cookie")
	reqest.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:12.0) Gecko/20100101 Firefox/12.0")

	response, err := client.Do(reqest) //提交

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func HTTPDelete(url_str string) error {
	client := &http.Client{}

	req, err := http.NewRequest("DELETE", url_str, nil)
	res, err := client.Do(req)
	if err != nil {
		return nil
	}

	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	return nil
}

func SendLog(test_type, logserver, data string) error {
	if logserver == "" {
		return nil
	}
	logdata := make(url.Values)
	logdata["s"] = []string{string(data)}
	logdata["n"] = []string{string(test_type)}
	_, err := HTTPPost(logserver, logdata)
	if err != nil {
		return err
	}
	return nil
}

func UpdateStatus(cache_server, test_id, status string) error {
	if cache_server == "" {
		return fmt.Errorf("Cache server not defined.")
	}
	put_data := make(url.Values)
	put_data["field"] = []string{"status"}
	put_data["value"] = []string{status}

	_, err := HTTPPut(cache_server+"/hash/"+test_id, put_data)
	if err != nil {
		return err
	}
	return nil
}

func UpdateRunningInfo(cache_server, test_id string, pid int) error {
	if cache_server == "" {
		return nil
	}
	put_data := make(url.Values)
	put_data["field"] = []string{"pid", "status"}
	put_data["value"] = []string{strconv.Itoa(pid) + ":0", "running"}

	_, err := HTTPPut(cache_server+"/hash/"+test_id, put_data)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil
}

func RedUpdatePodInfo_Start(cache_server, test_id, testfile string) error {
	if cache_server == "" {
		return nil
	}

	mem_data := make(url.Values)
	mem_data["key"] = []string{G_LOCALIP}
	mem_data["member"] = []string{test_id}

	_, err := HTTPPost(cache_server+"/set", mem_data)
	if err != nil {
		jsonString, _ := json.Marshal(mem_data)
		return fmt.Errorf("NetworkErr:%s-%s-%s",
			err, cache_server+"/set", jsonString)
	}

	put_data := make(url.Values)
	put_data["key"] = []string{G_LOCALIP + "_" + test_id}
	put_data["field"] = []string{"tid", "status", "testfile", "start_time"}
	put_data["value"] = []string{test_id, "running", testfile,
		fmt.Sprintf(time.Now().Format("2006-01-02 15:04:05"))}

	_, err = HTTPPost(cache_server+"/hash", put_data)
	if err != nil {
		jsonString, _ := json.Marshal(put_data)
		return fmt.Errorf("NetworkErr:%s-%s-%s",
			err, cache_server+"/hash/"+G_LOCALIP, jsonString)
	}
	return nil
}

func RedUpdatePodInfo_End(cache_server, testid string) error {
	if cache_server == "" {
		return nil
	}

	err := HTTPDelete(cache_server + "/key/" + G_LOCALIP + "_" + testid)
	if err != nil {
		return fmt.Errorf("NetworkErr:%s-%s-%s",
			err, cache_server+"/key/"+G_LOCALIP+"/"+testid)
	}

	put_data := make(url.Values)
	put_data["type"] = []string{"srem"}
	put_data["member"] = []string{testid}

	_, err = HTTPPut(cache_server+"/set/"+G_LOCALIP, put_data)
	if err != nil {
		return fmt.Errorf("NetworkErr:%s-%s-%s",
			err, cache_server+"/set/"+G_LOCALIP)
	}

	return nil
}

func RedUpdatePodInfo_Init(cache_server string) error {
	if cache_server == "" {
		return nil
	}

	err := HTTPDelete(cache_server + "/key/" + G_LOCALIP)
	if err != nil {
		return fmt.Errorf("%s-%s-%s",
			err, cache_server+"/key/"+G_LOCALIP)
	}

	return nil
}

func CheckCPDFRunning(cache_server string) (int, error) {
	setinfos, err := GetTestSet(cache_server)
	if err != nil {
		return -1, nil
	}
	type _valInfo struct {
		Ret string      `json:"_type"`
		Val interface{} `json:"val"`
	}

	msg := ""
	test_cnt := 0
	fmt.Println(setinfos.Val)
	for _, testid := range setinfos.Val {
		res, err := HTTPGet(cache_server + "/key?type=exists&key=" + testid)
		if err != nil {
			fmt.Println(err.Error())
			msg += fmt.Sprintf("%s Error;", err.Error())
			continue
		}
		var val _valInfo
		err = json.Unmarshal(res, &val)
		if err != nil {
			fmt.Println(err.Error())
			msg += fmt.Sprintf("%s Error;", err.Error())
			continue
		}
		//fmt.Println(msg)
		if val.Val.(bool) {
			res, err := HTTPGet(cache_server + "/hash?field=status&type=hget&key=" + testid)
			err = json.Unmarshal(res, &val)
			if err != nil {
				fmt.Println(err.Error())
				msg += fmt.Sprintf("%s Error;", err.Error())
				continue
			}

			if val.Val.(string) == "start" || val.Val.(string) == "running" {
				test_cnt++
			}
		} else {
			// Remove testid from CPDF_TESTID
			put_data := make(url.Values)
			put_data["type"] = []string{"srem"}
			put_data["member"] = []string{testid}

			_, err := HTTPPut(cache_server+"/set/CPDF_TESTID", put_data)
			if err != nil {
				msg += fmt.Sprintf(`Remove '%s' failed:'%s'`, testid, err.Error())
			}
		}
	}
	return test_cnt, fmt.Errorf(msg)

}

func UpdateStartInfo(cache_server, test_id, version, testcase, concurrence, interval, running_time, test_time, server string) error {
	// Save test_id to /set/CPDF_TESTID
	cache_id_data := make(url.Values)
	cache_id_data["key"] = []string{"CPDF_TESTID"}
	cache_id_data["member"] = []string{test_id}
	_, err := HTTPPost(cache_server+"/set", cache_id_data)
	if err != nil {
		return fmt.Errorf("RedisLinkError:%s", err.Error())
	}

	// Save test info to /hash/key
	cache_test_data := make(url.Values)
	cache_test_data["key"] = []string{test_id}
	cache_test_data["field"] = []string{
		"version",
		"test",
		"initial_concurrent",
		"interval",
		"running-time",
		"test-time",
		"server",
		"status",
		"start-time"}
	cache_test_data["value"] = []string{
		version,
		testcase,
		concurrence,
		interval,
		running_time,
		test_time,
		server,
		"start",
		time.Now().Format("2006-01-02 15:04:05")}

	fmt.Println(cache_test_data)

	_, err = HTTPPost(cache_server+"/hash", cache_test_data)
	if err != nil {
		return fmt.Errorf("RedisLinkError:%s", err.Error())
	}
	return nil
}

func UpdateExpire(cache_server, test_id string, exp_t int64) error {
	put_data := make(url.Values)
	put_data["expire"] = []string{strconv.FormatInt(exp_t*1e9, 10)}

	_, err := HTTPPut(cache_server+"/key/"+test_id, put_data)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil
}

func GetPid(cache_server, testid string) (string, error) {
	res, err := HTTPGet(cache_server + "/hash?key=" + testid + "&type=hget&field=pid")
	if err != nil {
		return "", fmt.Errorf("Get pid from redis error.")
	}
	type _pidData struct {
		Ret string `json:"_type"`
		Pid string `json:"val"`
	}

	var data _pidData
	err = json.Unmarshal(res, &data)
	if err != nil {
		return "", fmt.Errorf("JSONErr:%s", err.Error())
	}

	return data.Pid, nil
}

func DeleteTestInfo(cache_server, testid string) error {
	err := HTTPDelete(cache_server + "/key/" + testid)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	put_data := make(url.Values)
	put_data["type"] = []string{"srem"}
	put_data["member"] = []string{testid}

	_, err = HTTPPut(cache_server+"/set/CPDF_TESTID", put_data)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	return nil
}

type TestInfo struct {
	initial_concurrent string
	interval           string
	running_time       string
	server             string
	test               string
	test_time          string
	version            string
	start_time         string
}

func GetTestInfo(cache_server string, testid []string) ([]byte, error) {
	type _info struct {
		Ret                string `json:"_type"`
		Initial_concurrent string `json:"initial-concurrent"`
		Interval           string `json:"interval"`
		Running_time       string `json:"running-time"`
		Server             string `json:"server"`
		Test               string `json:"test"`
		Test_time          string `json:"test-time"`
		Version            string `json:"version"`
		Start_time         string `json:"start-time"`
		Status             string `json:"status"`
		Pid                string `json:"pid"`
	}

	type _testInfo struct {
		Testid string `json:"testid"`
		Info   _info  `json:"info"`
	}

	//var testInfo _testInfo
	testInfos := []_testInfo{}
	for _, t := range testid {
		res, err := HTTPGet(cache_server + "/hash?key=" + t)
		if err != nil {
			return nil, err
		}
		fmt.Println(string(res))
		var info _info
		err = json.Unmarshal(res, &info)
		if err != nil {
			fmt.Println(err.Error())
			return nil, err
		}
		fmt.Println(info)
		if info.Ret == "3" {
			continue
		}
		var testinfo _testInfo
		testinfo.Testid = t
		testinfo.Info = info
		testInfos = append(testInfos, testinfo)

	}
	if len(testInfos) == 0 {
		return nil, fmt.Errorf("EMPTY")
	}
	ret, err := json.Marshal(testInfos)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

type _setInfo struct {
	Val []string `json:"val"`
}

func GetTestSet(cache_server string) (_setInfo, error) {
	var setInfo _setInfo
	res, err := HTTPGet(cache_server + "/set?key=CPDF_TESTID")
	if err != nil {
		return setInfo, err
	}
	err = json.Unmarshal(res, &setInfo)
	if err != nil {
		return setInfo, err
	}
	return setInfo, nil
}

func Run(test_type,
	testfile_str,
	program,
	program_param,
	timeout_s,
	forcekill_process_str,
	custom_param string) (log_str []byte, err error) {

	var timeout_i time.Duration = 0
	if timeout_s != "" {
		timeout_tem, _err := strconv.Atoi(timeout_s)
		if _err == nil {
			timeout_i = (time.Duration)(timeout_tem)
		} else {
			err = fmt.Errorf("timeout set error")
			return
		}
	}

	var cmd_str string
	//	status_info := "success"
	if runtime.GOOS == "windows" {
		cmd_str = "cmd.exe /c call " + program + " " + program_param
	} else {
		cmd_str = program + " " + program_param
	}

	cmd_list := strings.Split(cmd_str, " ")
	cmd_list = append(cmd_list, "--custom_param="+custom_param)

	cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
	stdout, exec_err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("STDOUT ERROR:", exec_err.Error())
		err = exec_err
		return
	}

	exec_err = cmd.Start()
	if exec_err != nil {
		if strings.Index(exec_err.Error(), "permission denied") != -1 {
			cmd := exec.Command("chmod", "+x", program)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()

			cmd = exec.Command(cmd_list[0], cmd_list[1:]...)
			stdout, exec_err = cmd.StdoutPipe()
			if err != nil {
				fmt.Println("STDOUT ERROR:", exec_err.Error())
				err = exec_err
				return
			}
			exec_err = cmd.Start()
			if exec_err != nil {
				fmt.Println(exec_err.Error())
				err = exec_err
				return
			}
		}
		//fmt.Println("ERROR:", err.Error())
		err = exec_err
		return
	}

	bexited := false
	bkill := false
	go func() {
		if int(timeout_i) > 0 {
			time.Sleep(timeout_i * time.Second)
			if bexited {
				return
			}
			bkill = true
			_err := Kill(strconv.Itoa(cmd.Process.Pid), 0)
			if _err != nil {
				//				fmt.Fprintf(w, "%v", fmt.Sprintf(`{"ret":-2,"err":"%s"}`, err.Error()))
				fmt.Println(err.Error())
				err = _err
				return
			}
			if forcekill_process_str != "" {
				forcekill_process := strings.Split(forcekill_process_str, ",")
				for _, f_kill := range forcekill_process {
					KillByName(f_kill)
				}
			}

			err = fmt.Errorf("death")

		}
	}()

	log_str, exec_err = ioutil.ReadAll(stdout)
	if exec_err != nil {
		err = exec_err
		return
	}

	exec_err = cmd.Wait()
	bexited = cmd.ProcessState.Exited()
	if exec_err != nil {
		err = exec_err
		if bkill {
			err = fmt.Errorf("Death")
		} else {
			if forcekill_process_str != "" {
				forcekill_process := strings.Split(forcekill_process_str, ",")
				for _, f_kill := range forcekill_process {
					fmt.Println("Kill :", f_kill)
					KillByName(f_kill)
				}
			}
		}
		return
	}

	return
}

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

func GetLocalIP(filter string) (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		// handle err
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
				//			case *net.IPAddr:
				//				ip = v.IP
			}

			if ip.String() == "127.0.0.1" ||
				ip == nil ||
				strings.Count(ip.String(), ".") != 3 {
				continue
			}
			if filter != "" {
				if !strings.Contains(ip.String(), filter) {
					continue
				}
			}

			return ip, nil
		}
	}
	return nil, fmt.Errorf("Not found IP")
}
