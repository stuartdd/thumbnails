package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type TNResp struct {
	returnCode int
	mimeType   string
	resp       []byte
}

var (
	NF     = &TNResp{returnCode: http.StatusNotFound, mimeType: "application/json", resp: []byte(`{"message":"Not Found"}`)}
	CLOSED = &TNResp{returnCode: http.StatusOK, mimeType: "application/json", resp: []byte(`{"message":"Server-Closed"}`)}
)

type TNServer struct {
	port      int
	server    *http.Server
	getRoutes map[string]func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp
	srcPath   string
}

func controlHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {
	if uri[0] == "close" {
		go func() {
			time.Sleep(1 * time.Second)
			tns.Close()
		}()
		return CLOSED
	}
	return NF
}

func imageHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {
	if uri[0] == "tn" {
		srcFile := tns.convertToPath(uri)
		pic := NewPicture(srcFile)
		if pic.err != nil {
			logError("EXIF  :", srcFile, pic.err)
			return NF
		}
		return &TNResp{returnCode: http.StatusOK, mimeType: "application/json", resp: []byte(`{"message":"OK"}`)}
	}
	return NF
}

func NewTnServer(port int, srcPath string) *TNServer {
	tns := &TNServer{port: port, getRoutes: make(map[string]func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp)}
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rq := strings.Split(r.RequestURI, "/")
			if len(rq) > 1 && rq[0] == "" {
				rq = rq[1:]
			}

			if r.Method == http.MethodGet {
				fn, found := tns.getRoutes[rq[0]]
				var resp *TNResp
				if found && len(rq) > 1 {
					resp = fn(rq[1:], tns, w, r)
				}
				if resp == nil {
					resp = NF
				}
				writeResp(tns, w, resp)
			} else {
				writeResp(tns, w, NF)
			}
		}),
	}
	tns.AddGetHandler("control", controlHandler)
	tns.AddGetHandler("image", imageHandler)
	tns.server = srv
	return tns
}

func writeResp(tns *TNServer, w http.ResponseWriter, resp *TNResp) {
	w.WriteHeader(NF.returnCode)
	w.Header().Set("Content-Type", NF.mimeType)
	_, _ = w.Write(NF.resp)
}

func (s *TNServer) AddGetHandler(uri string, handle func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp) {
	s.getRoutes[uri] = handle
}

func (s *TNServer) convertToPath(uri []string) string {
	var sb strings.Builder
	sb.WriteString(s.srcPath)
	sb.WriteRune(os.PathSeparator)
	for i, p := range uri {
		sb.WriteString(p)
		if (i < len(uri))
		sb.WriteRune(os.PathSeparator)

	}
}

func (s *TNServer) Close() {
	s.server.Close()
}

func (s *TNServer) Run() error {
	return s.server.ListenAndServe()
}
