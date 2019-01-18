package main

import (
	"fmt"
	"net/http"
	"os"
)

func (this *LogService) Init() error {
	if this.logPath != "" {
		f, err := os.OpenFile(this.logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModeAppend|os.ModePerm)
		if err != nil {
			return err
		}
		this.logFileHandle = make(map[string]*os.File)
		this.logFileHandle["main"] = f
	}
	return nil
}

func (this *LogService) LogWrite(w http.ResponseWriter, r *http.Request) {
	fmt.Println("LogWrite")
	r.ParseMultipartForm(32 << 20)

	logname := "main"
	name := GetFormValue(w, r, "n")
	if name != "" {
		logname = name
	}

	log_str := GetFormValue(w, r, "s")
	if log_str == "" {
		fmt.Fprintf(w, "%v", `{"type":"error","msg":"log string null"}`)
		return
	}
	this.logFileHandle[logname].WriteString(log_str + "\n")
}

func (this *LogService) LogRegister(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	name := GetFormValue(w, r, "n")
	if name != "" {
		fmt.Fprintf(w, "%v", `{"type":"error","msg":"name undefined"}`)
		return
	}

	logpath := GetFormValue(w, r, "p")
	if logpath == "" {
		fmt.Fprintf(w, "%v", `{"type":"error","msg":"path undefine"}`)
		return
	}
	force_r := GetFormValue(w, r, "p")

	if _, ok := this.logFileHandle["name"]; ok {
		if force_r != "1" {
			fmt.Fprintf(w, "%v", `{"type":"error","msg":"name exists. replace it set 'f=1'"}`)
			return
		}
	}

	f, err := os.OpenFile(this.logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModeAppend|os.ModePerm)
	if err != nil {
		fmt.Fprintf(w, "%v", `{"type":"error","msg":"create log file failed"}`)
		return
	}

	this.logFileHandle[name] = f
	fmt.Fprintf(w, "%v", `{"type":"success","msg":""}`)
}
func (this *LogService) Info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%v", "FoxitQA Test Server.")
}
