package main

import (
	"path/filepath"
	//	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"strconv"
	"strings"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/BurntSushi/toml"

	"github.com/bitly/go-simplejson"
	fxqacommon "github.com/xiaoxiayu/services/fxqa-tool/common"
	"github.com/xiaoxiayu/services/fxqa-tool/route"

	cmap "github.com/streamrail/concurrent-map"
)

type gatewayConfig struct {
	Version       string
	RouteCfg      string
	Port          int
	EtcdEndPoints string
	FileServer    string
	Kubernetes    K8sInfo
}

type ServiceInfo struct {
	ServiceBaseName string
	Routes          []RouteInfo
	Port            int
}

type RouteInfo struct {
	Path   string
	Method string
}

type K8sInfo struct {
	Server    string
	Port      int
	NodeLabel string
}

func ReadCfg() gatewayConfig {
	cfg_paths := []string{"/etc/fxqa-gateway.conf", "fxqa-gateway.conf"}
	cfg := flag.String("cfg", "", "Configure file.")
	flag.Parse()
	if *cfg != "" {
		cfg_paths = append(cfg_paths, *cfg)
	}
	var config gatewayConfig
	//	fmt.Println(cfg_path)
	for _, cfg_path := range cfg_paths {
		if _, err := toml.DecodeFile(cfg_path, &config); err != nil {
			log.Println(err)
			continue
		}
		log.Println("Configure:", cfg_path)
		WatchConfig(cfg_path)
		break
	}

	return config
}

var g_nodeHashRing *fxqacommon.Consistent
var g_service_infos cmap.ConcurrentMap

func init() {
	g_nodeHashRing = fxqacommon.NewConsisten()
	g_service_infos = cmap.New()
}

func RegisterRoute(server_name, version string, service_port int, routes []RouteInfo) {
	for _, route_info := range routes {
		fmt.Println(route_info.Method, "/"+version+"/"+server_name+route_info.Path)
		var _ = route.Register("/"+version+"/"+server_name+route_info.Path,
			route_info.Method,
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")

				client := &http.Client{}
				//service_url := GetServiceHost(service_name)
				url_str := fmt.Sprintf("http://%s:%d%s",
					g_nodeHashRing.Get(fxqacommon.RandomNumeric(2)),
					service_port,
					strings.Replace(r.URL.String(), "/"+version+"/"+server_name, "", 1))
				//				////////DEBUG
				//				url_str = fmt.Sprintf("http://%s:%d%s",
				//					"127.0.0.1",
				//					9093,
				//					strings.Replace(r.URL.String(), "/"+server_name, "", 1))
				//				url_str = strings.Replace(url_str, "/"+version, "", 1)
				//				fmt.Println(url_str)
				//				////////DEBUG
				var body io.Reader
				if r.Method == "GET" || r.Method == "DELETE" {
					body = nil
				} else {
					body_s, _ := ioutil.ReadAll(r.Body)
					body = strings.NewReader(string(body_s))
				}

				req, err := http.NewRequest(r.Method, url_str, body)
				if err != nil {
					return
				}
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

				res, err := client.Do(req)
				if err != nil {
					return
				}

				res_body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return
				}
				w.WriteHeader(res.StatusCode)
				fmt.Fprintf(w, "%v", string(res_body))
			})
	}
}

func WatchConfig(cfg_path string) {
	go fxqacommon.WatcheFile(cfg_path, func() {
		var cfg gatewayConfig
		if _, err := toml.DecodeFile(cfg_path, &cfg); err == nil {
			DoGateWay(cfg.Kubernetes, cfg.Version, cfg.RouteCfg)
		} else {
			log.Println(err)
		}
	})
}

func RegisterSwagger(host, service_name, swagger_info_path string) {
	fmt.Println("/swagger-ui/" + service_name)
	route.Register("/swagger-ui/"+service_name, "GET",
		func(w http.ResponseWriter, r *http.Request) {
			t, err := template.ParseFiles("./swagger-index-template.html")
			if err != nil {
				fmt.Println(err.Error())
			}
			data := struct {
				DataUrl string
				Host    string
			}{
				DataUrl: swagger_info_path,
				Host:    host,
			}
			t.Execute(w, data)
		})
}

func ParseGateWay(k8s_info K8sInfo, version, swagger_info_path string) {
	for service_map := range g_service_infos.IterBuffered() {
		service_name := service_map.Key
		service_info := service_map.Val.(ServiceInfo)

		RegisterRoute(service_name, version, service_info.Port, service_info.Routes)
		RegisterSwagger(g_nodeHashRing.Get(fxqacommon.RandomString(4)),
			service_name, swagger_info_path)
	}

}

func ParseRouteInfo(cfg_json_path string) (*simplejson.Json, error) {
	fmt.Println("Get Swagger File From:", cfg_json_path)
	body, err := fxqacommon.HTTPGet(cfg_json_path)
	if err != nil {
		return nil, err
	}

	route_js, err := simplejson.NewJson(body)

	service_info := ServiceInfo{}

	basePath, _ := route_js.Get("basePath").String()

	routeInfos := []RouteInfo{}
	paths, _ := route_js.Get("paths").Map()

	for path_key, path_info := range paths {
		for method_key, _ := range path_info.(map[string]interface{}) {
			routeInfo := RouteInfo{Method: method_key, Path: path_key}
			routeInfos = append(routeInfos, routeInfo)
		}
	}
	service_info.Routes = routeInfos
	g_service_infos.Set(basePath[1:], service_info)
	fmt.Println(basePath)

	return route_js, nil
}

func UpdateServerInfo(k8s_info K8sInfo) error {
	nodes, err := fxqacommon.GetNodes("http://"+
		k8s_info.Server+":"+strconv.Itoa(k8s_info.Port)+
		"/api/v1/nodes", k8s_info.NodeLabel)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	for _, node_url := range nodes {
		g_nodeHashRing.Add(node_url)
	}
	for service_map := range g_service_infos.IterBuffered() {
		service_name := service_map.Key

		selector_str := strings.Replace(service_name, "/", ":", -1)
		port, err := fxqacommon.GetServicePortFromSelector("http://"+
			k8s_info.Server+":"+strconv.Itoa(k8s_info.Port)+
			"/api/v1/services", selector_str)
		if err != nil {
			fmt.Println("http://"+
				k8s_info.Server+":"+strconv.Itoa(k8s_info.Port)+
				"/api/v1/services "+selector_str, err.Error())
			return err
		}
		service_info := service_map.Val.(ServiceInfo)
		service_info.Port = int(port)
		g_service_infos.Set(service_name, service_info)
	}

	return nil
}

func GetK8sGatewayServiceHost(k8s_info K8sInfo) (string, error) {
	selector_str := "platform:gateway"
	port, err := fxqacommon.GetServicePortFromSelector("http://"+
		k8s_info.Server+":"+strconv.Itoa(k8s_info.Port)+
		"/api/v1/services", selector_str)
	if err != nil {
		return "", err
	}

	server_ip := g_nodeHashRing.Get(fxqacommon.RandomNumeric(2))
	fmt.Println("SERVER IP:", server_ip, "PORT:", port)
	return server_ip + ":" + strconv.Itoa(int(port)), nil
}

func UpdateGatewayServiceCfg(k8s_info K8sInfo, route_js *simplejson.Json, version string) (*simplejson.Json, error) {

	basePath, err := route_js.Get("basePath").String()
	if err != nil {
		return nil, err
	}
	host_str, err := GetK8sGatewayServiceHost(k8s_info)
	if err != nil {
		return nil, err
	}
	route_js.Set("host", host_str)

	route_js.Set("basePath", "/"+version+basePath)

	return route_js, nil
}

func CreateNewCfg(route_js *simplejson.Json, interface_version, cfg_path string) (string, error) {
	b, _ := route_js.Encode()
	err := ioutil.WriteFile("tem.tem", b, 0777)
	file_name := interface_version + "_" + filepath.Base(cfg_path)
	new_url := strings.Replace(cfg_path, filepath.Base(cfg_path), file_name, -1)

	save_path := strings.Split(new_url, ":9090/")[1]
	err = fxqacommon.QAFileServerUpload(G_FILE_SERVER+":9091",
		"tem.tem", save_path)
	if err != nil {
		return "", err
	}
	new_path := G_FILE_SERVER + ":9090/" + save_path

	return new_path, err
}

func DoGateWay(k8s_info K8sInfo, interface_version, swagger_cfg string) error {
	route_cfg_js, err := ParseRouteInfo(swagger_cfg)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	err = UpdateServerInfo(k8s_info)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	new_route_cfg_js, _ := UpdateGatewayServiceCfg(k8s_info, route_cfg_js, interface_version)

	new_cfg, err := CreateNewCfg(new_route_cfg_js, interface_version, swagger_cfg)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	fmt.Println("Upload Swagger:", new_cfg)
	ParseGateWay(k8s_info, interface_version, new_cfg)
	return nil
}

func WatchService(etcd_key string, cfg gatewayConfig) error {
	kapi, err := fxqacommon.EtcdInit(cfg.EtcdEndPoints)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	watcher := kapi.Watcher(etcd_key, &client.WatcherOptions{
		Recursive: true,
	})
	getres, err := kapi.Get(context.Background(), etcd_key,
		&client.GetOptions{Quorum: true, Recursive: true})
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	if getres.Node.Dir {
		for _, child_node := range getres.Node.Nodes {
			//// Local Debug
			//			DoGateWay(cfg.Kubernetes, cfg.Version,
			//				G_FILE_SERVER+":9090/ServicesCfg/"+"cache.json")
			//			continue
			//// Debug End

			DoGateWay(cfg.Kubernetes, cfg.Version,
				G_FILE_SERVER+":9090/"+child_node.Value)

		}
	}

	for {
		res, err := watcher.Next(context.Background())
		if err != nil {
			log.Println("Error watch workers:", err)
			break
		}

		if res.Action == "set" {
			fmt.Println(res.Node.Key)
			val, err := kapi.Get(context.Background(), res.Node.Key, &client.GetOptions{Quorum: true})
			if err != nil {
				fmt.Println(err.Error())
			} else {
				//ParseGateWay(cfg.Kubernetes, cfg.Version, val.Node.Value)
				DoGateWay(cfg.Kubernetes, cfg.Version,
					G_FILE_SERVER+":9090/"+val.Node.Value)
			}
		}

	}
	return nil
}

var G_FILE_SERVER string

func main() {
	cfg := ReadCfg()
	G_FILE_SERVER = "http://" + cfg.FileServer

	route.Dir("/swagger-base", "swagger")

	go WatchService("/services", cfg)

	fmt.Println("==========START========")
	route.Start(cfg.Port)
	fmt.Println("==========EXIT=========")
}
