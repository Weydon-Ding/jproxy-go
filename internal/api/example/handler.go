package example

import (
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
	"net/http"
	"strings"
)

type Store interface{ Repositories() sqlite.Repositories }

type Options struct {
	Store    Store
	Provider runtime.Provider
	Domain   string
}

type Handler struct{ options Options }

func NewHandler(options Options) *Handler { return &Handler{options: options} }

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/api/"+h.options.Domain+"/example")
	switch request.Method + " " + path {
	case http.MethodPost + " /save":
		h.save(writer, request)
	case http.MethodGet + " /query":
		h.query(writer, request)
	case http.MethodPost + " /remove":
		h.remove(writer, request)
	default:
		if method := allowedMethod(path); method != "" {
			writer.Header().Set("Allow", method)
			writeError(writer, http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(writer, request)
	}
}

func allowedMethod(path string) string {
	switch path {
	case "/query":
		return http.MethodGet
	case "/save", "/remove":
		return http.MethodPost
	default:
		return ""
	}
}
