package main

import (
	"fmt"
	"image/jpeg"
	"io/fs"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stuartdd2/JsonParser4go/parser"
)

type UserData struct {
	userName  string
	locations map[string]string
}

type TNServer struct {
	port          int
	server        *http.Server
	thumbNailSize int
	getRoutes     map[string]func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp
	srcPath       string
	verbose       bool
	startTime     int64
	users         map[string]*UserData
}

type TNResp struct {
	returnCode int
	mimeType   string
	resp       []byte
}

const (
	MEDIA_JSON = "application/json"
	PATH_SEP   = string(os.PathSeparator)
	URL_SEP    = "/"
	NL         = "\n"
)

var (
	CLOSED = &TNResp{returnCode: http.StatusOK, mimeType: MEDIA_JSON, resp: []byte(`{"message":"Server-Closed"}` + NL)}

	THUMB_FILE_TYPES = map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
	}

	THUMB_FILE_TYPE = ".jpg"
	USER_PATH       = parser.NewDotPath("resources.users")
)

func UMT(tag, ent string, uri []string, err error) *TNResp {
	logError(fmt.Sprintf("UnsupportedMediaType:%s: ent:%s", tag, ent), strings.Join(uri, URL_SEP), err)
	return &TNResp{returnCode: http.StatusUnsupportedMediaType, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"message\":\"Unsupported Media Type\", \"Item\": \"%s\"\"}", ent))}
}

func ISE(tag, ent string, uri []string, err error) *TNResp {
	logError(fmt.Sprintf("InternalServerError:%s: ent:%s", tag, ent), strings.Join(uri, URL_SEP), err)
	return &TNResp{returnCode: http.StatusInternalServerError, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"message\":\"Internal Server Error\", \"Item\": \"%s\"\"}", ent))}
}

func NF(tag string, uri []string, err error) *TNResp {
	logError(fmt.Sprintf("NotFound:%s:", tag), strings.Join(uri, URL_SEP), err)
	return &TNResp{returnCode: http.StatusNotFound, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"message\":\"Not Found\", \"URI\": \"%s\"\"}", strings.Join(uri, URL_SEP)))}
}

func BR(tag, ent string, uri []string, err error) *TNResp {
	logError(fmt.Sprintf("BadRequest:%s: ent:%s", tag, ent), strings.Join(uri, URL_SEP), err)
	return &TNResp{returnCode: http.StatusBadRequest, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"message\":\"Bad Request\", \"Value\": \"%s\"\"}", ent))}
}

func (s *TNResp) String() string {
	return fmt.Sprintf("{\"RESPONSE\":{\"rc\":\"%d\",\"mime\":\"%s\",\"len\":\"%d\",\"resp\":\"%s\" }}", s.returnCode, s.mimeType, len(s.resp), encodeString(s.resp, 50, s.mimeType))
}

/*
   Backspace is replaced with \b.
   Form feed is replaced with \f.
   Newline is replaced with \n.
   Carriage return is replaced with \r.
   Tab is replaced with \t.
   Double quote is replaced with \"
   Backslash is replaced with \\
*/
func encodeString(b []byte, max int, mt string) string {
	image := strings.Contains(mt, "image")
	var sb strings.Builder
	for i, c := range b {
		if image {
			sb.WriteRune('[')
			if c < 16 {
				sb.WriteRune('0')
			}
			sb.WriteString(strconv.FormatInt(int64(c), 16))
			sb.WriteRune(']')
		} else {
			switch c {
			case 8:
				sb.WriteRune('\\')
				sb.WriteRune('b')
			case 12:
				sb.WriteRune('\\')
				sb.WriteRune('f')
			case 10:
				sb.WriteRune('\\')
				sb.WriteRune('n')
			case 13:
				sb.WriteRune('\\')
				sb.WriteRune('r')
			case 11:
				sb.WriteRune('\\')
				sb.WriteRune('t')
			case 34:
				sb.WriteRune('\\')
				sb.WriteRune('"')
			case 92:
				sb.WriteRune('\\')
				sb.WriteRune('\\')
			default:
				if c >= 32 && c <= 127 {
					sb.WriteByte(c)
				}
			}
		}
		if i > 25 && image {
			break
		} else {
			if i > max {
				break
			}
		}
	}
	return sb.String()
}

func NewTnServer(port int, srcPath, configPath string, sizeInt int, verbose bool) (*TNServer, error) {
	absFileName, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	j, err := ioutil.ReadFile(absFileName)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(j)) == "" {
		return nil, fmt.Errorf("file '%s' is empty", absFileName)
	}
	configData, err := parser.Parse(j)
	if err != nil {
		return nil, err
	}
	if verbose {
		log.Printf("{\"CONFIG\":{\"file\":\"%s\",\"info\":\"Parsed config ok\"}}", absFileName)
	}
	userDataNode, err := parser.Find(configData, USER_PATH)
	if err != nil {
		return nil, err
	}
	userDataObj, ok := userDataNode.(*parser.JsonObject)
	if !ok {
		return nil, fmt.Errorf("config data [%s] node %s is not a json object", absFileName, USER_PATH.String())
	}
	userMap := make(map[string]*UserData)
	for _, ud := range userDataObj.GetValues() {
		name := ud.GetName()
		udObj, ok := ud.(*parser.JsonObject)
		if !ok {
			return nil, fmt.Errorf("user data node %s.%s is not an object node", USER_PATH, name)
		}
		locations := make(map[string]string)
		for _, udv := range udObj.GetValues() {
			udvStr, ok := udv.(*parser.JsonString)
			if ok {
				locations[udvStr.GetName()] = udvStr.GetValue()
			}
		}
		userMap[ud.GetName()] = &UserData{userName: name, locations: locations}
		if verbose {
			n := ud.(parser.NodeC).GetNodeWithName("name")
			if n != nil {
				log.Printf("{\"CONFIG\":{\"user\":\"%s\",\"info\":\"%s\"}}", ud.GetName(), n)
			} else {
				log.Printf("{\"CONFIG\":{\"user\":\"%s\",\"info\":\"undefined\"}}", ud.GetName())
			}
		}
	}

	routes := make(map[string]func([]string, *TNServer, http.ResponseWriter, *http.Request) *TNResp)
	tns := &TNServer{port: port, srcPath: srcPath, getRoutes: routes, thumbNailSize: sizeInt, verbose: verbose, users: userMap, startTime: time.Now().Unix()}
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.RawPath
			if p == "" {
				p = r.URL.Path
			}
			rq := strings.Split(p, URL_SEP)
			if len(rq) > 1 && rq[0] == "" {
				rq = rq[1:]
			}
			if r.Method == http.MethodGet {
				if verbose {
					log.Printf("{\"REQUEST\":{\"path\":\"%s\", \"query\":\"%s\"}}", p, r.URL.RawQuery)
				}
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
	tns.AddGetHandler("files", fileHandler)
	tns.AddGetHandler("paths", pathHandler)
	tns.server = srv
	if verbose {
		log.Printf("{\"SERVER\":{\"port\":\"%d\",\"info\":\"Configured\"}}", port)
	}
	return tns, nil
}

//
// /paths/user/{user}/loc/{loc} to filepath
//
func locationFromPath(uri []string, tns *TNServer) (string, *TNResp) {
	user := dataFromPathElement(uri, "user")
	if user == "" {
		return "", NF("PATH", uri, nil)
	}
	loc := dataFromPathElement(uri, "loc")
	if loc == "" {
		return "", NF("PATH", uri, nil)
	}
	location := filepath.Join(tns.srcPath, tns.users[user].locations[loc])
	_, err := os.Stat(location)
	if err != nil {
		return "", ISE("PATH", location, uri, err)
	}
	return location, nil
}

func filePathFromPath(uri []string, location string, tns *TNServer, pathRequired bool) (string, bool, *TNResp) {
	path := dataFromPathElement(uri, "path")
	if path == "" {
		if pathRequired {
			return "", false, NF("PATH", uri, nil)
		}
	}
	unEscapedPath, err := url.PathUnescape(path)
	if err != nil {
		return "", false, BR("PATH", path, uri, err)
	}

	if unEscapedPath == "." {
		unEscapedPath = ""
	}

	fullPath := ""
	name := dataFromPathElement(uri, "name")
	if name == "" {
		fullPath = filepath.Join(location, unEscapedPath)
	} else {
		unEscapedName, err := url.PathUnescape(name)
		if err != nil {
			return "", false, BR("PATH", name, uri, err)
		}
		fullPath = filepath.Join(location, unEscapedPath, unEscapedName)
	}
	if !strings.HasPrefix(fullPath, location) {
		return "", false, BR("PATH", "invalid-path", uri, nil)
	}
	fil, err := os.Stat(fullPath)
	if err != nil {
		return "", false, NF("PATH", uri, nil)
	}
	return fullPath, fil.IsDir(), nil
}

//
// paths/user/{user}/loc/{loc}
//
func pathHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {
	location, resp := locationFromPath(uri, tns)
	if resp != nil {
		return resp
	}

	path, isDir, resp := filePathFromPath(uri, location, tns, false)
	if resp != nil {
		return resp
	}

	if !isDir {
		return BR("PATH", "not-dir", uri, nil)
	}

	var sb strings.Builder
	count := 0
	sb.WriteString("[")
	fsys := os.DirFS(path)
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if d != nil {
			if d.IsDir() && p != "." {
				fp := filepath.Join(path, p)
				if len(filesOfInterest(fp, queryAllFile(r))) > 0 {
					sb.WriteString(fmt.Sprintf("\n  \"%s\",", url.PathEscape(p+PATH_SEP)))
					count++
				}
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

//
//
// files/user/{user}/loc/{loc}/path/{path}
//
func fileHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {
	location, resp := locationFromPath(uri, tns)
	if resp != nil {
		return resp
	}
	if len(uri) < 5 {
		return NF("FILE", uri, nil)
	}

	path, isDir, resp := filePathFromPath(uri, location, tns, true)
	if resp != nil {
		return resp
	}

	if isDir {
		return returnFileList(path, queryAllFile(r))
	}

	thumbnail, thumbNailSize := queryThumbnail(r, tns.thumbNailSize)
	return returnFileContent(path, uri, thumbnail, thumbNailSize, tns)
}

func returnFileList(path string, all bool) *TNResp {
	list := filesOfInterest(path, all)

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

func returnFileContent(srcFile string, uri []string, thumbnail bool, thumbNailSize int, tns *TNServer) *TNResp {
	ext := filepath.Ext(srcFile)
	_, fName := filepath.Split(srcFile)

	if thumbnail {
		_, ok := THUMB_FILE_TYPES[ext]
		if !ok {
			return UMT("FILE", fName, uri, nil)
		}
		pic := NewPicture(srcFile, thumbnail)
		if pic.err != nil {
			return NF("IMAGE", uri, pic.err)
		}
		dstImage, err := createThumbImage(pic, "", thumbNailSize, tns.verbose, true, len(tns.srcPath))
		if err != nil {
			return ISE("THUMB", pic.GetFileName(), uri, err)
		}
		w := NewEncodedWriter(500)
		err = jpeg.Encode(w, dstImage, &jpeg.Options{Quality: jpeg.DefaultQuality})
		if err != nil {
			return ISE("ENCODE", pic.GetFileName(), uri, err)
		}
		return &TNResp{returnCode: http.StatusOK, mimeType: THUMB_FILE_TYPES[THUMB_FILE_TYPE], resp: w.Bytes()}
	}

	mediaType := mime.TypeByExtension(ext)
	buf, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return ISE("FILE", fName, uri, err)
	}
	return &TNResp{returnCode: http.StatusOK, mimeType: mediaType, resp: buf}
}

func controlHandler(uri []string, tns *TNServer, w http.ResponseWriter, r *http.Request) *TNResp {
	if uri[0] == "close" {
		go func() {
			time.Sleep(2 * time.Second)
			tns.Close()
		}()
		if tns.verbose {
			log.Printf("{\"SERVER\":{\"port\":\"%d\",\"info\":\"Stopping\"}}", tns.port)
		}
		return CLOSED
	}
	if uri[0] == "time" {
		return &TNResp{returnCode: http.StatusOK, mimeType: MEDIA_JSON, resp: []byte(fmt.Sprintf("{\"upSeconds\": \"%d\"}", time.Now().Unix()-tns.startTime))}
	}
	return NF("CNTL", uri, nil)
}

func writeResp(tns *TNServer, w http.ResponseWriter, resp *TNResp) {
	if tns.verbose {
		log.Print(resp)
	}
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
		log.Printf("{\"SERVER\":{\"port\":\"%d\",\"info\":\"Stopped\"}}", s.port)
	}
	s.server.Close()
}

func (s *TNServer) Run() error {
	if s.verbose {
		log.Printf("{\"SERVER\":{\"port\":\"%d\",\"info\":\"Started\"}}", s.port)
	}
	return s.server.ListenAndServe()
}

func queryThumbnail(r *http.Request, defSize int) (bool, int) {
	tnRaw := r.URL.Query().Get("thumbnail")
	tn := strings.TrimSpace(tnRaw)
	if tn == "" {
		return false, 0
	}
	i, err := strconv.Atoi(tn)
	if err != nil {
		return true, defSize
	}
	return true, i
}

func queryAllFile(r *http.Request) bool {
	tnRaw := r.URL.Query().Get("allfiles")
	tn := strings.TrimSpace(tnRaw)
	return tn != ""
}

func dataFromPathElement(uri []string, name string) string {
	for i, s := range uri {
		if s == name && len(uri) > i+1 {
			return uri[i+1]
		}
	}
	return ""
}

func filesOfInterest(path string, all bool) []string {
	list := make([]string, 0)
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return list
	}
	for _, f := range files {
		if !strings.HasPrefix(f.Name(), ".") {
			if !f.IsDir() {
				mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(f.Name())))
				if all || strings.HasPrefix(mt, "image/") {
					list = append(list, f.Name())
				}
			}
		}
	}
	return list
}
