package main

import (
	"fmt"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type TNResp struct {
	returnCode int
	mimeType   string
	resp       []byte
}

var (
	RE         = &TNResp{returnCode: http.StatusUnsupportedMediaType, mimeType: "application/json", resp: []byte(`{"message":"Read Error"}` + "\n")}
	NF         = &TNResp{returnCode: http.StatusNotFound, mimeType: "application/json", resp: []byte(`{"message":"Not Found"}` + "\n")}
	CLOSED     = &TNResp{returnCode: http.StatusOK, mimeType: "application/json", resp: []byte(`{"message":"Server-Closed"}` + "\n")}
	FILE_TYPES = map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".png":  "image/png",
		".tiff": "image/tiff",
		".tif":  "image/tiff",
		".svg":  "image/svg+xml",
		".ico":  "image/vnd.microsoft.icon",
	}
)

type EncodedWriter struct {
	bytes []byte
	pos   int
	ext   int
	size  int
}

func NewEncodedWriter(ext int) *EncodedWriter {
	if ext < 2 {
		panic("Encoded Writer extension must be more than 1")
	}
	b := make([]byte, ext)
	return &EncodedWriter{bytes: b, pos: 0, ext: ext, size: len(b)}
}

func (ew *EncodedWriter) Bytes() []byte {
	return ew.bytes[0:ew.pos]
}

func (ew *EncodedWriter) Write(p []byte) (n int, err error) {
	pos := ew.pos
	for _, b := range p {
		if pos >= ew.size {
			ew.bytes = append(ew.bytes, make([]byte, ew.ext)...)
			ew.size = len(ew.bytes)
		}
		ew.bytes[pos] = b
		pos++
	}
	ew.pos = pos
	return len(p), nil
}

type TNServer struct {
	port      int
	server    *http.Server
	getRoutes map[string]func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp
	srcPath   string
	verbose   bool
}

func controlHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {
	if uri[0] == "close" {
		go func() {
			time.Sleep(2 * time.Second)
			tns.Close()
		}()
		if tns.verbose {
			log.Printf("Stop server requested: port:%d", tns.port)
		}
		return CLOSED
	}
	return NF
}

func imageHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {
	srcFile := tns.convertToPath(uri[1:])
	pic := NewPicture(srcFile)
	if pic.err != nil {
		logError("EXIF  :", srcFile, pic.err)
		return NF
	}
	if uri[0] == "full" {
		b, err := ioutil.ReadFile(srcFile)
		if err != nil {
			return RE
		}
		mt, ok := FILE_TYPES[pic.ext]
		if !ok {
			return RE
		}
		return &TNResp{returnCode: http.StatusOK, mimeType: mt, resp: b}
	} else {
		si, err := strconv.Atoi(uri[0])
		if err != nil {
			logError("Invalid thunbnail size:", fmt.Sprintf("Value:%s URI:%s", uri[0], strings.Join(uri, "/")), err)
			return RE
		}
		if si < 10 {
			logError("Invalid thunbnail size (less than 10):", fmt.Sprintf("Value:%s URI:%s", uri[0], strings.Join(uri, "/")), err)
			return RE
		}

		w := NewEncodedWriter(500)
		dstImage, err := createThumbImage(pic, srcFile, si, tns.verbose)
		if err != nil {
			return RE
		}
		err = jpeg.Encode(w, dstImage, &jpeg.Options{Quality: jpeg.DefaultQuality})
		if err != nil {
			logError("ENCODE:", srcFile, err)
			return RE
		}
		return &TNResp{returnCode: http.StatusOK, mimeType: FILE_TYPES[".jpg"], resp: w.Bytes()}
	}
}

func NewTnServer(port int, srcPath string, verbose bool) *TNServer {
	routes := make(map[string]func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp)
	tns := &TNServer{port: port, srcPath: srcPath, getRoutes: routes, verbose: verbose}
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
	if verbose {
		log.Printf("Created server: port:%d, src:%s", port, srcPath)
	}
	return tns
}

func writeResp(tns *TNServer, w http.ResponseWriter, resp *TNResp) {
	w.WriteHeader(resp.returnCode)
	w.Header().Set("Content-Type", resp.mimeType)
	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(resp.resp)))
	_, _ = w.Write(resp.resp)
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
		if i < (len(uri) - 1) {
			sb.WriteRune(os.PathSeparator)
		}
	}
	return sb.String()
}

func (s *TNServer) Close() {
	if s.verbose {
		log.Printf("Stopping server: port:%d", s.port)
	}
	s.server.Close()
}

func (s *TNServer) Run() error {
	if s.verbose {
		log.Printf("Starting server: port:%d", s.port)
	}
	return s.server.ListenAndServe()
}
