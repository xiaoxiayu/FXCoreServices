package route

import (
	"strconv"

	"net/http"
	_ "net/http/pprof"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var g_router *mux.Router

func Register(route_str, methods string,
	body func(w http.ResponseWriter, r *http.Request)) error {
	g_router.HandleFunc(route_str, body).Methods(methods)
	return nil
}

func Dir(prefix, dir_path string) {
	g_router.PathPrefix(prefix).
		Handler(http.StripPrefix(prefix, handlers.CORS()(http.FileServer(http.Dir(dir_path)))))
}

func init() {
	g_router = mux.NewRouter()
}

func Start(port int) error {
	http.Handle("/", g_router)
	methods := handlers.AllowedMethods(
		[]string{"DELETE", "GET", "HEAD", "POST", "PUT", "OPTIONS"})
	http.ListenAndServe(":"+strconv.Itoa(port), handlers.CORS(methods)(g_router))
	return nil
}
