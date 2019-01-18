// database.go
package main

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func (this *HttpHander) DataPost(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	r.ParseMultipartForm(32 << 20)

	fileID := this.GetFormValue(w, r, "FileID")
	if fileID == "" {
		fmt.Fprintf(w, "%v", "ERROR:FileID invalid.")
		return
	}

	relative_path := this.GetFormValue(w, r, "FilePath") // actully like file path.
	if relative_path == "" {
		fmt.Fprintf(w, "%v", "ERROR:FilePath invalid.")
		return
	}
	file_path := Upload_Dir + relative_path

	fileName := filepath.Base(file_path)
	storePath := filepath.Dir(file_path)
	extName := path.Ext(fileName)
	extName = strings.ToLower(extName)

	fileSize_str := this.GetFormValue(w, r, "FileSize")
	fmt.Println(fileSize_str)
	if fileSize_str == "" {
		fmt.Fprintf(w, "%v", "ERROR:DBGetFileSize invalid.")
		return
	}
	fileSize, _ := strconv.ParseInt(string(fileSize_str), 10, 64)

	Info := this.GetFormValue(w, r, "Info")

	ret, db_str := this.db.InsertTestFile(fileID, storePath, fileName, fileSize, extName, Info)
	if ret != 0 {
		fmt.Fprintf(w, "%v%s", "DBERROR:", db_str)
		return
	}
}

func (this *HttpHander) DataGet(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	r.ParseMultipartForm(32 << 20)

	selectStr := this.GetFormValue(w, r, "s")
	if selectStr == "" {
		fmt.Fprintf(w, "%v", `{"type":"error", "info":"search invalid"}`)
		return
	}

	limitStr := this.GetFormValue(w, r, "l")
	if limitStr == "" {
		fmt.Fprintf(w, "%v", `{"type":"error", "info":"limit invalid"}`)
		return
	}

	whereStr := this.GetFormValue(w, r, "c")
	//	if whereStr == "" {
	//		fmt.Fprintf(w, "%v", `{"type":"error", "info":"limit invalid"}`)
	//		return
	//	}
	//fmt.Println("SELECT")
	seclect_json, err := this.db.Select(selectStr, whereStr, limitStr)
	if err != nil {
		fmt.Fprintf(w, "%v", err.Error())
		return
	}
	fmt.Fprintf(w, "%v", seclect_json)
}

func (this *HttpHander) FileGet(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	r.ParseMultipartForm(32 << 20)

	fileID := this.GetFormValue(w, r, "id")
	if fileID == "" {
		fmt.Fprintf(w, "%v", `{"type":"error", "info":"filename invalid, key:'id'"}`)
		return
	}

	//fmt.Println("**Check DB.**")
	ret, already_path := this.db.GetTestFilePath(fileID)
	if ret == -1 {
		fmt.Fprintf(w, "%v", fmt.Sprintf(`{"type":"success", "info":"%s"}`, already_path))
		return
	}
	fmt.Fprintf(w, "%v", "0")

}

func (this *HttpHander) Info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%v", "Database service.")
}
