package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

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

	//time.Sleep(time.Duration(G_Interface_SleepTime * 1e9))
	return string(body), nil
}

func Run(test_type, cmd_str, log_server string) error {
	if runtime.GOOS == "windows" {
		fmt.Println(cmd_str)
		cmd_list := strings.Split(cmd_str, " ")

		cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
		buf, err := cmd.Output()
		err = cmd.Start()
		if err != nil {
			if strings.Contains(err.Error(), "not found") {

			} else {

			}
		} else {
			fmt.Println("process start")
		}

		cmd.Run()
		cmd.Wait()

		if log_server == "" {
			return nil
		}
		logdata := make(url.Values)
		logdata["s"] = []string{string(buf)}
		logdata["n"] = []string{test_type}
		_, err = HTTPPost(log_server, logdata)
		if err != nil {
			return err
			//			fmt.Fprintf(w, "%v", `{"type":"error","msg":"create log file failed"}`)
		}
		//fmt.Println(res)

	} else {

		//		cmd_list := strings.Split(cmd_str, " ")
		//fmt.Printf("== RUN:%s ==\n", cmd_str)
		cmd_list := strings.Split(cmd_str, " ")

		cmd := exec.Command(cmd_list[0], cmd_list[1:]...)
		buf, err := cmd.Output()

		if err != nil {
			return err
			//			fmt.Fprintf(w, "%v", fmt.Sprintf(`{"type":"error","msg":"%s","test":"%s","testfile":"%s"}`, err.Error(), test, testfile))
		}
		cmd.Run()
		cmd.Wait()

		if log_server == "" {
			return nil
		}

		logdata := make(url.Values)
		logdata["s"] = []string{string(buf)}
		logdata["n"] = []string{test_type}
		_, err = HTTPPost(log_server, logdata)
		if err != nil {
			return err
		}

	}
	return nil
}
