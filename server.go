package main

import (
	"fmt"
	"image/jpeg"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type TNResp struct {
	returnCode int
	mimeType   string
	resp       []byte
}

const (
	MEDIA_JSON = "application/json"
)

var (
	CLOSED     = &TNResp{returnCode: http.StatusOK, mimeType: MEDIA_JSON, resp: []byte(`{"message":"Server-Closed"}` + "\n")}
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

func UMT(tag, ent string, uri []string, err error) *TNResp {
	logError(fmt.Sprintf("UnsupportedMediaType:%s:", tag), strings.Join(uri, "/"), err)
	return &TNResp{returnCode: http.StatusUnsupportedMediaType, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"message\":\"Unsupported Media Type\", \"Entity\": \"%s\"\"}", ent))}
}

func ISE(tag, ent string, uri []string, err error) *TNResp {
	logError(fmt.Sprintf("InternalServerError:%s:", tag), strings.Join(uri, "/"), err)
	return &TNResp{returnCode: http.StatusInternalServerError, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"message\":\"Internal Server Error\", \"Entity\": \"%s\"\"}", ent))}
}

func NF(tag string, uri []string, err error) *TNResp {
	logError(fmt.Sprintf("NotFound:%s:", tag), strings.Join(uri, "/"), err)
	return &TNResp{returnCode: http.StatusNotFound, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"message\":\"Not Found\", \"URI\": \"%s\"\"}", strings.Join(uri, "/")))}
}

func BR(tag, ent string, uri []string, err error) *TNResp {
	logError(fmt.Sprintf("BadRequest:%s:", tag), strings.Join(uri, "/"), err)
	return &TNResp{returnCode: http.StatusBadRequest, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"message\":\"Bad Request\", \"Value\": \"%s\"\"}", ent))}
}

type TNServer struct {
	port      int
	server    *http.Server
	getRoutes map[string]func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp
	srcPath   string
	verbose   bool
	startTime time.Time
}

func fileSystemHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {

	if uri[0] == "tree" {
		var sb strings.Builder
		count := 0
		sb.WriteString("[")
		fsys := os.DirFS(tns.srcPath)
		fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if d.IsDir() && p != "." {
				fp := fmt.Sprintf("%s%c%s", tns.srcPath, os.PathSeparator, p)
				if len(filesOfInterest(fp)) > 0 {
					sb.WriteString(fmt.Sprintf("\n  \"%s\",", url.PathEscape(p+"/")))
					count++
				}
			}
			return nil
		})
		s := sb.String()
		if count > 0 {
			s = s[:len(s)-1]
		}
		return &TNResp{returnCode: http.StatusOK, mimeType: MEDIA_JSON, resp: []byte(s + "\n]")}
	}

	if len(uri) < 2 {
		return BR("FILE", "Not enough data", uri, nil)
	}

	unEscapedPathVar, err := url.PathUnescape(uri[1])
	if err != nil {
		return BR("FILE", "Malformed path", uri, err)
	}

	if uri[0] == "list" {
		path := fmt.Sprintf("%s%c%s", tns.srcPath, os.PathSeparator, unEscapedPathVar)
		list := filesOfInterest(path)

		var sb strings.Builder
		count := 0
		sb.WriteString("[")
		for _, f := range list {
			sb.WriteString(fmt.Sprintf("\n  \"%s\",", url.PathEscape(f)))
			count++
		}
		s := sb.String()
		if count > 0 {
			s = s[:len(s)-1]
		}
		return &TNResp{returnCode: http.StatusOK, mimeType: MEDIA_JSON, resp: []byte(s + "\n]")}
	}
	return NF("FILE", uri, nil)
}

func imageHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {
	if len(uri) < 2 {
		return BR("IMAGE", "Not enough data", uri, nil)
	}
	unEscapedPathVar, err := url.PathUnescape(uri[1])
	if err != nil {
		return BR("IMAGE", "Malformed path", uri, err)
	}

	srcFile := fmt.Sprintf("%s%c%s", tns.srcPath, os.PathSeparator, unEscapedPathVar)
	if uri[0] == "full" {
		pic := NewPicture(srcFile, false)
		if pic.err != nil {
			return NF("IMAGE", uri, pic.err)
		}
		b, err := ioutil.ReadFile(srcFile)
		if err != nil {
			return ISE("IMAGE", pic.GetFileName(), uri, pic.err)
		}
		mt, ok := FILE_TYPES[pic.ext]
		if !ok {
			return UMT("IMAGE", pic.GetFileName(), uri, pic.err)
		}
		return &TNResp{returnCode: http.StatusOK, mimeType: mt, resp: b}
	} else {
		pic := NewPicture(srcFile, true)
		if pic.err != nil {
			return UMT("IMAGE", pic.GetFileName(), uri, pic.err)
		}

		si, err := strconv.Atoi(uri[0])
		if err != nil {
			return BR("IMAGE", fmt.Sprintf("size '%s' is invalid", uri[0]), uri, err)
		}
		if si < 10 {
			return BR("IMAGE", fmt.Sprintf("size '%s' below 10", uri[0]), uri, err)
		}

		w := NewEncodedWriter(500)
		dstImage, err := createThumbImage(pic, srcFile, si, tns.verbose)
		if err != nil {
			return ISE("THUMB", pic.GetFileName(), uri, err)
		}
		err = jpeg.Encode(w, dstImage, &jpeg.Options{Quality: jpeg.DefaultQuality})
		if err != nil {
			return ISE("ENCODE", pic.GetFileName(), uri, err)
		}
		return &TNResp{returnCode: http.StatusOK, mimeType: FILE_TYPES[".jpg"], resp: w.Bytes()}
	}
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
	if uri[0] == "time" {
		return &TNResp{returnCode: http.StatusOK, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"up\": \"%s\"}", tns.startTime))}
	}
	return NF("CNTL", uri, nil)
}

func NewTnServer(port int, srcPath string, verbose bool) *TNServer {
	routes := make(map[string]func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp)
	tns := &TNServer{port: port, srcPath: srcPath, getRoutes: routes, verbose: verbose, startTime: time.Now()}
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
					resp = NF("GET", rq, nil)
				}
				writeResp(tns, w, resp)
			} else {
				writeResp(tns, w, NF("UNSUPPORTED", rq, nil))
			}
		}),
	}
	tns.AddGetHandler("control", controlHandler)
	tns.AddGetHandler("image", imageHandler)
	tns.AddGetHandler("files", fileSystemHandler)
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

func filesOfInterest(path string) []string {
	list := make([]string, 0)
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return list
	}
	for _, f := range files {
		_, ok := FILE_TYPES[strings.ToLower(filepath.Ext(f.Name()))]
		if ok {
			list = append(list, f.Name())
		}
	}
	return list
}
