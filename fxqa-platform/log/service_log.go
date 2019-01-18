package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/natefinch/lumberjack"
)

func (this *LogService) Init() error {
	fmt.Println("------")
	this.logFileHandle = make(map[string]*log.Logger)
	for logName, logInfo := range this.cfg.Log {
		if !logInfo.Enable {
			continue
		}
		fmt.Println(logInfo.Path)
		logger := log.New(nil, "", 0)
		logger.SetOutput(&lumberjack.Logger{
			Filename:   logInfo.Path,
			MaxSize:    logInfo.MaxSize, // megabytes
			MaxBackups: logInfo.MaxBackup,
			MaxAge:     logInfo.MaxAge, //days
		})

		this.logFileHandle[logName] = logger
		fmt.Println(logName, logger)

		now := time.Now()
		this.logFileHandle[logName].Println(`Log Start:`,
			now.Format("2006-01-02 15:04:05"))
		_, err := os.Stat(logInfo.Path)
		if err != nil && os.IsNotExist(err) {
			log.Panic("Error: can not create logfile:", logInfo.Path)
		}
		go func() {
			for true {
				_, err := os.Stat(logInfo.Path)
				if err != nil && os.IsNotExist(err) {
					logger := log.New(nil, "", 0)
					logger.SetOutput(&lumberjack.Logger{
						Filename:   logInfo.Path,
						MaxSize:    logInfo.MaxSize, // megabytes
						MaxBackups: logInfo.MaxBackup,
						MaxAge:     logInfo.MaxAge, //days
					})

					this.logFileHandle[logName] = logger
					fmt.Println(logName, logger)

					now := time.Now()
					this.logFileHandle[logName].Println(`Log Start:`,
						now.Format("2006-01-02 15:04:05"))
					_, err := os.Stat(logInfo.Path)
					if err != nil && os.IsNotExist(err) {
						log.Panic("Error: can not create logfile:", logInfo.Path)
					}
				}
				time.Sleep(1e5)
			}

		}()
	}

	return nil
}

func (this *LogService) LogWrite(w http.ResponseWriter, r *http.Request) {
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

	if this.logFileHandle[logname] == nil {
		fmt.Fprintf(w, "%v", `{"type":"error","msg":"log name not found"}`)
		return
	}

	this.logFileHandle[logname].Println(log_str)
}

func (this *LogService) LogRegister(w http.ResponseWriter, r *http.Request) {
	//	r.ParseMultipartForm(32 << 20)

	//	name := GetFormValue(w, r, "n")
	//	if name != "" {
	//		fmt.Fprintf(w, "%v", `{"type":"error","msg":"name undefined"}`)
	//		return
	//	}

	//	logpath := GetFormValue(w, r, "p")
	//	if logpath == "" {
	//		fmt.Fprintf(w, "%v", `{"type":"error","msg":"path undefine"}`)
	//		return
	//	}
	//	force_r := GetFormValue(w, r, "f")

	//	if _, ok := this.logFileHandle["name"]; ok {
	//		if force_r != "1" {
	//			fmt.Fprintf(w, "%v", `{"type":"error","msg":"name exists. replace it set 'f=1'"}`)
	//			return
	//		}
	//	}

	//	f, err := os.OpenFile(this.logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModeAppend|os.ModePerm)
	//	if err != nil {
	//		fmt.Fprintf(w, "%v", `{"type":"error","msg":"create log file failed"}`)
	//		return
	//	}

	//	this.logFileHandle[name] = f
	fmt.Fprintf(w, "%v", `{"type":"unsupport","msg":"unsupport"}`)
}
