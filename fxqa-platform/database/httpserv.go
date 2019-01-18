// database.go
package main

import (
	"fmt"
	"net/http"
)

//type Myhandler struct{}
type home struct {
	Title string
}

type HttpHander struct {
	db *DBHander
}

func (this *HttpHander) Init(db_ip string, k8sapi_ip string, cache_ports []string) error {
	this.db = new(DBHander)
	node_ips, err := GetNodes("http://" + k8sapi_ip + "/api/v1/nodes")
	if err != nil {
		fmt.Println("Get Nodes Error:%s", err.Error())
		return err
	}

	err = this.db.Init(db_ip, cache_ports, node_ips)
	if err != nil {
		return err
	}
	return nil
}

func (this *HttpHander) GetFormValue(w http.ResponseWriter, r *http.Request, form_name string) string {
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

func (this *HttpHander) SignIn(w http.ResponseWriter, r *http.Request) {
	fmt.Println("SignIn")
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	r.ParseMultipartForm(32 << 20)
	user := this.GetFormValue(w, r, "user")
	if user == "" {
		fmt.Fprintf(w, "%v", "ERROR:user invalid.")
		return
	}
	fmt.Println(user)
	pwd, err := this.db.SelectUserPwd(user)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Fprintf(w, "%v", "ERROR:Already Exist.")
		return
	}
	fmt.Fprintf(w, "%v", fmt.Sprintf(`{"PWD":"%s"}`, pwd))
}

func (this *HttpHander) SignOut(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	r.ParseMultipartForm(32 << 20)
	user := this.GetFormValue(w, r, "user")
	if user == "" {
		fmt.Fprintf(w, "%v", "ERROR:user invalid.")
		return
	}
	fmt.Fprintf(w, "%v", `{"result":"ok"}`)

	pwd := this.GetFormValue(w, r, "pwd")
	if pwd == "" {
		fmt.Fprintf(w, "%v", "ERROR:pwd invalid.")
		return
	}
}

func (this *HttpHander) SignUp(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	r.ParseMultipartForm(32 << 20)
	user := this.GetFormValue(w, r, "user")
	if user == "" {
		fmt.Fprintf(w, "%v", "ERROR:user invalid.")
		return
	}

	email := this.GetFormValue(w, r, "email")
	if email == "" {
		fmt.Fprintf(w, "%v", "ERROR:pwd invalid.")
		return
	}

	pwd := this.GetFormValue(w, r, "pwd")
	if pwd == "" {
		fmt.Fprintf(w, "%v", "ERROR:pwd invalid.")
		return
	}

	err := this.db.InsertUser(user, email, pwd)
	if err != nil {
		fmt.Fprintf(w, "%v", "ERROR:Already Exist.")
		return
	}
	fmt.Fprintf(w, "%v", `0`)
}

func (this *HttpHander) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST,PUT")

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	r.ParseMultipartForm(32 << 20)
	user := this.GetFormValue(w, r, "user")
	if user == "" {
		fmt.Fprintf(w, "%v", "ERROR:user invalid.")
		return
	}
	fmt.Fprintf(w, "%v", `{"result":"ok"}`)

	pwd := this.GetFormValue(w, r, "pwd")
	if pwd == "" {
		fmt.Fprintf(w, "%v", "ERROR:pwd invalid.")
		return
	}

}
