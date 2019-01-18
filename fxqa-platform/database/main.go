// server.go
package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/gorilla/mux"
	//"github.com/gorilla/securecookie"
)

var Upload_Dir string
var Backup_Dir string
var G_USERINFO_EXPIRE_TIME string
var G_DATA_SELECT_EXPIRE_TIME string

func ReadCfg() (string, string, []string) {
	database_ip := flag.String("database", "", "Database Server IP.")
	apiserver_ip := flag.String("apiserver", "", "K8s api server IP. ip:port")
	ports := flag.String("ports", "", "Platform cache service ports. port0;port1")
	flag.Parse()

	if *database_ip == "" || *apiserver_ip == "" || *ports == "" {
		fmt.Println("Config ERROR.")
		os.Exit(0)
	}
	cache_ips := strings.Split(*ports, ";")

	return *database_ip, *apiserver_ip, cache_ips
}

var memprofile = flag.String("memprofile", "", "write memory profile to this file")

var router = mux.NewRouter()

// k8s service port: 32456
func main() {
	G_USERINFO_EXPIRE_TIME = "2592000"   // 30 Days
	G_DATA_SELECT_EXPIRE_TIME = "432000" // 5 Days

	database_ip, apiserver_ip, cache_ips := ReadCfg()

	http_serv := new(HttpHander)
	err := http_serv.Init(database_ip, apiserver_ip, cache_ips)
	if err != nil {
		fmt.Println("Database Init Error.")
		return
	}

	router.HandleFunc("/info", http_serv.Info).Methods("GET")

	router.HandleFunc("/data", http_serv.DataPost).Methods("POST")
	router.HandleFunc("/data", http_serv.DataGet).Methods("GET")
	//	router.HandleFunc("/cache-server", http_serv.CacheServerAdd).Methods("POST")

	router.HandleFunc("/file", http_serv.FileGet).Methods("GET")

	router.HandleFunc("/signin", http_serv.SignIn).Methods("POST")
	router.HandleFunc("/signout", http_serv.SignOut).Methods("POST")
	router.HandleFunc("/signup", http_serv.SignUp).Methods("POST")
	http.Handle("/", router)
	http.ListenAndServe(":9090", nil)
}
