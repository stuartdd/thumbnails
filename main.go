package main

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liujiawm/graphics-go/graphics"
	"github.com/rwcarlsen/goexif/exif"
)

type Picture struct {
	source      string
	name        string
	ext         string
	orientation int
	err         error
	time        time.Time
	modTime     time.Time
}

const (
	TIME_FORMAT_1 = "2006:01:02 15:04:05"
	TIME_FORMAT_2 = "2006-01-02T15:04:05"
	TIME_FORMAT_3 = "20060102_150405"
	NAME_MASK     = "%YYYY_%MM_%DD_%h_%m_%s_%n.%x"

	NC_ARG            = "noclobber"
	VB_ARG            = "verbose"
	HELP_ARG          = "help"
	MASK_ARG          = "mask="
	SIZE_ARG          = "size="
	LOG_FILE_ARG      = "logfile="
	SERVER_PORT_ARG   = "serverport="
	SERVER_CONFIG_ARG = "serverconfig="

	HELP_HINT = ". Use 'help' option to view usage"
)

func main() {

	if findBoolArg(HELP_ARG, true) {
		exitWithHelp("", 0)
	}
	if len(os.Args) < 3 {
		log.Fatalf("Not enough args [%d]%s", len(os.Args)-1, HELP_HINT)
	}
	srcPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatalf("Source path '%s' is invalid %s%s", os.Args[1], err.Error(), HELP_HINT)
	}
	sizeInt, err := findIntArg(SIZE_ARG, 10, 1000, 200)
	if err != nil {
		log.Fatalf("Invalid size option. Requires an int from 10..1000. %s%s", err.Error(), HELP_HINT)
	}
	serverPort, err := findIntArg(SERVER_PORT_ARG, -1, 9999999, -1)

	var logFileWriter *LFWriter

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Printf("{\"SERVER\":{\"port\":\"%d\",\"info\":\"captured %v, stopping server and exiting rc=1..\"}}", serverPort, sig)
			if logFileWriter != nil {
				logFileWriter.CloseLogWriter()
			}
			time.Sleep(time.Second)
			os.Exit(1)
		}
	}()

	logFile := findStringArg(LOG_FILE_ARG, "")
	if logFile != "" {
		logFileWriter, err = NewLFWriter(logFile, 60, func(s1, s2 string, err error) {
			log.Printf("{\"LOGGER\":{\"type\":\"%s\",\"file\":\"%s\",\"error\":\"%s\"}}", s1, s2, EncodeString([]byte(err.Error()), 999, "json"))
		})
		if err != nil {
			log.Fatalf("Could not create timed log file. Name:%s Error:%e%s", logFile, err, HELP_HINT)
		}
		log.SetOutput(logFileWriter)
		log.Printf("{\"LOGGER\":{\"type\":\"INFO\",\"text\":\"Created\"}}")
	}
	defer func() {
		if logFileWriter != nil {
			logFileWriter.CloseLogWriter()
		}
	}()

	verbose := findBoolArg(VB_ARG, true)
	if serverPort > -1 {
		configDataFile := findStringArg(SERVER_CONFIG_ARG, "")
		if configDataFile == "" {
			log.Fatalf("Config data arg [%s] is not defined.", SERVER_CONFIG_ARG)
		}
		tns, configErr := NewTnServer(serverPort, srcPath, configDataFile, sizeInt, verbose)
		if configErr != nil {
			log.Fatalf("Config data [%s] error '%s'.", configDataFile, configErr.Error())
		}
		err := tns.Run()
		if err != nil {
			if err != http.ErrServerClosed {
				log.Fatal("Server Error: " + err.Error())
			} else {
				if verbose {
					log.Printf("{\"SERVER\":{\"port\":\"%d\",\"info\":\"Closed\"}}", serverPort)
				}
			}
		}
		return
	}
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		log.Fatalf("Source path:%s%s", err.Error()[5:], HELP_HINT)
	}
	if !srcInfo.IsDir() {
		log.Fatalf("Source path '%s%s' must be a directory.", srcPath, HELP_HINT)
	}
	dstPath, err := filepath.Abs(os.Args[2])
	if err != nil {
		log.Fatalf("Destination path '%s%s' is invalid %s", os.Args[2], err.Error(), HELP_HINT)
	}
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		log.Fatalf("Destination path:%s%s", err.Error()[5:], HELP_HINT)
	}
	if !dstInfo.IsDir() {
		log.Fatalf("Destination path '%s%s' must be a directory.", srcPath, HELP_HINT)
	}

	fileNameMask := findStringArg(MASK_ARG, NAME_MASK)
	noClobber := findBoolArg(NC_ARG, true)

	filepath.Walk(srcPath, func(inPath string, info fs.FileInfo, errIn error) error {
		if !info.IsDir() {
			relPath, _ := filepath.Split(inPath[len(srcPath):])
			relPath = filepath.Clean(relPath)
			outPath := filepath.Clean(fmt.Sprintf("%s%s", dstPath, relPath))
			_, err = os.Stat(outPath)
			if err != nil {
				os.MkdirAll(outPath, os.ModePerm)
			}
			thumb(inPath, outPath, fileNameMask, sizeInt, noClobber, verbose)
		}
		return nil
	})
}

func NewPicture(source string, thumbnail bool) *Picture {
	_, fName := filepath.Split(source)
	ext := filepath.Ext(strings.ToLower(fName))
	name := fName[0 : len(fName)-len(ext)]

	stat, err := os.Stat(source)
	if err != nil {
		return &Picture{source: source, name: name, ext: ext, orientation: 1, modTime: time.Now(), time: time.Now(), err: err}
	}
	modTime := stat.ModTime()
	picTime, err := timeParseStr(name)
	if err != nil {
		picTime = modTime
	}

	f, err := os.Open(source)
	if err != nil {
		return &Picture{source: source, name: name, ext: ext, orientation: 1, modTime: modTime, time: picTime, err: err}
	}
	defer f.Close()
	if thumbnail {
		x, err := exif.Decode(f)
		if err != nil {
			return &Picture{source: source, name: name, ext: ext, orientation: 1, modTime: modTime, time: picTime, err: err}
		}
		i, err := x.Get(exif.Orientation)
		if err != nil {
			return &Picture{source: source, name: name, ext: ext, orientation: 1, modTime: modTime, time: picTime, err: err}
		}
		iv, err := i.Int(0)
		if err != nil {
			return &Picture{source: source, name: name, ext: ext, orientation: 1, modTime: modTime, time: picTime, err: err}
		}

		t, err := timeParseX(x, exif.DateTimeOriginal)
		if err != nil {
			t, err = timeParseX(x, exif.DateTimeDigitized)
			if err != nil {
				t, err = timeParseX(x, exif.DateTime)
				if err != nil {
					t, err = timeParseStr(name)
					if err != nil {
						t = modTime
					}
				}
			}
		}
		return &Picture{source: source, name: name, ext: ext, orientation: iv, modTime: modTime, time: t, err: nil}
	}
	return &Picture{source: source, name: name, ext: ext, orientation: 1, modTime: modTime, time: modTime, err: nil}
}

func (p *Picture) GetFileName() string {
	return p.name + p.ext
}

func timeParseX(ex *exif.Exif, field exif.FieldName) (time.Time, error) {
	v, err := ex.Get(field)
	if err != nil {
		return time.Now(), err
	}
	s, err := v.StringVal()
	if err != nil {
		return time.Now(), err
	}
	return timeParseStr(s)
}

func timeParseStr(strTime string) (time.Time, error) {
	st := strings.TrimSpace(strTime)
	if st == "" {
		return time.Now(), fmt.Errorf("empty time string")
	}
	t, err := time.Parse(TIME_FORMAT_1, strTime)
	if err != nil {
		t, err = time.Parse(TIME_FORMAT_2, strTime)
		if err != nil {
			t, err = time.Parse(TIME_FORMAT_3, strTime)
			if err != nil {
				return time.Now(), err
			}
		}
	}
	return t, nil
}

func createThumbImage(pic *Picture, thumbName string, size int, verbose bool, server bool, srcPrefix int) (*image.RGBA, error) {
	imagePath, err := os.Open(pic.source)
	if err != nil {
		logServer("OPEN", pic.source, err)
		return nil, err
	}
	defer imagePath.Close()
	srcImage, _, err := image.Decode(imagePath)
	if err != nil {
		logServer("DECODE", pic.source, err)
		return nil, err
	}
	b := srcImage.Bounds()
	var sh int
	var sw int
	if b.Dx() > b.Dy() {
		sh = size
		sw = int(float64(sh) * (float64(b.Dx()) / float64(b.Dy())))
	} else {
		sw = size
		sh = int(float64(sw) * (float64(b.Dy()) / float64(b.Dx())))
	}

	if verbose {
		if server {
			log.Printf("{\"THUMB\":{\"w\":\"%d\",\"h\":\"%d\",\"orientation\":\"%d\",\"source\":\"%s\"}}", sw, sh, pic.orientation, strings.ReplaceAll(pic.source[srcPrefix+1:], "\"", "\\\""))
		} else {
			if thumbName == "" {
				logServer("INFO", fmt.Sprintf("W:%d H:%d Orientation:%d in:%s", sw, sh, pic.orientation, pic.source), nil)
			} else {
				logServer("INFO", fmt.Sprintf("W:%d H:%d Orientation:%d in:%s: out:%s", sw, sh, pic.orientation, pic.source, thumbName), nil)
			}
		}
	}

	dstImage := image.NewRGBA(image.Rect(0, 0, sw, sh))
	err = graphics.Thumbnail(dstImage, srcImage)
	if err != nil {
		logServer("THUMB", pic.source, err)
		return nil, err
	}

	if pic.orientation != 1 {
		// 1 Upright
		// 6 rotate clockwise 90
		// 8 rotate anticlockwise 90
		// 3 Upside Down
		switch pic.orientation {
		case 6:
			rotImage := image.NewRGBA(image.Rect(0, 0, sh, sw))
			rotErr := graphics.Rotate(rotImage, dstImage, &graphics.RotateOptions{Angle: 1.5708}) // 90
			if rotErr != nil {
				logServer("ROTATE 90", pic.source, rotErr)
			} else {
				dstImage = rotImage
			}
		case 3:
			rotImage := image.NewRGBA(image.Rect(0, 0, sw, sh))
			rotErr := graphics.Rotate(rotImage, dstImage, &graphics.RotateOptions{Angle: 3.14159}) // 180
			if rotErr != nil {
				logServer("ROTATE 180", pic.source, rotErr)
			} else {
				dstImage = rotImage
			}
		case 8:
			rotImage := image.NewRGBA(image.Rect(0, 0, sh, sw))
			rotErr := graphics.Rotate(rotImage, dstImage, &graphics.RotateOptions{Angle: 4.71239}) // 270
			if rotErr != nil {
				logServer("ROTATE 270", pic.source, rotErr)
			} else {
				dstImage = rotImage
			}
		}
	}
	return dstImage, nil
}

func thumb(srcFile, thumbPath, thumbNameMask string, size int, noClobber, verbose bool) {
	pic := NewPicture(srcFile, true)
	if pic.err != nil {
		logServer("EXIF", srcFile, pic.err)
	}
	thumbFileName := fmt.Sprintf("%s%c%s", thumbPath, filepath.Separator, subFileName(pic.time, thumbNameMask, pic.name, "jpg"))
	if noClobber {
		_, err := os.Stat(thumbFileName)
		if err == nil {
			return
		}
	}

	dstImage, err := createThumbImage(pic, thumbFileName, size, verbose, false, 0)
	if err != nil {
		return
	}

	newImage, err := os.Create(thumbFileName)
	if err != nil {
		logServer("CREATE", thumbFileName, err)
		return
	}
	defer newImage.Close()

	err = jpeg.Encode(newImage, dstImage, &jpeg.Options{Quality: jpeg.DefaultQuality})
	if err != nil {
		logServer("ENCODE", thumbFileName, err)
		return
	}
}

func subFileName(time time.Time, mask, name, ext string) string {
	mfn := mask
	if strings.Contains(mfn, "%YYYY") {
		mfn = strings.ReplaceAll(mfn, "%YYYY", strPad4(time.Year()))
	}
	if strings.Contains(mfn, "%MM") {
		mfn = strings.ReplaceAll(mfn, "%MM", strPad2(int(time.Month())))
	}
	if strings.Contains(mfn, "%DD") {
		mfn = strings.ReplaceAll(mfn, "%DD", strPad2(int(time.Day())))
	}
	if strings.Contains(mfn, "%h") {
		mfn = strings.ReplaceAll(mfn, "%h", strPad2(time.Hour()))
	}
	if strings.Contains(mfn, "%m") {
		mfn = strings.ReplaceAll(mfn, "%m", strPad2(time.Minute()))
	}
	if strings.Contains(mfn, "%s") {
		mfn = strings.ReplaceAll(mfn, "%s", strPad2(time.Second()))
	}
	if strings.Contains(mfn, "%n") {
		mfn = strings.ReplaceAll(mfn, "%n", name)
	}
	if strings.Contains(mfn, "%x") {
		mfn = strings.ReplaceAll(mfn, "%x", ext)
	}
	return mfn
}

func strPad2(i int) string {
	if i < 10 {
		return "0" + strconv.Itoa(i)
	}
	return strconv.Itoa(i)
}

func strPad4(i int) string {
	if i > 999 {
		return strconv.Itoa(i)
	}
	if i > 99 {
		return "0" + strconv.Itoa(i)
	}
	if i > 9 {
		return "00" + strconv.Itoa(i)
	}
	return "000" + strconv.Itoa(i)
}

func logServer(tag, info string, err error) {
	if err == nil {
		log.Printf("{\"SERVER\":{\"type\":\"%s\",\"info\":\"%s\"}}", tag, info)
	} else {
		log.Printf("{\"SERVER\":{\"type\":\"%s\",\"info\":\"%s\",\"error\":\"%s\"}}", tag, info, err.Error())
	}
}

func findStringArg(prefix, def string) string {
	for _, v := range os.Args {
		if strings.HasPrefix(v, prefix) {
			return v[len(prefix):]
		}
	}
	return def
}

func findBoolArg(prefix string, found bool) bool {
	for _, v := range os.Args {
		if v == prefix {
			return found
		}
	}
	return !found
}

func findIntArg(prefix string, min, max, def int) (int, error) {
	argStr := findStringArg(prefix, fmt.Sprintf("%d", def))
	argInt, err := strconv.Atoi(argStr)
	if err != nil {
		return 0, fmt.Errorf("error: %s%s argument is invalid %s", prefix, argStr, err.Error())
	}
	if argInt < min {
		return 0, fmt.Errorf("error: %s%s argument is less than %d", prefix, argStr, min)
	}
	if argInt > max {
		return 0, fmt.Errorf("error: %s%s argument is more than %d", prefix, argStr, max)
	}
	return argInt, nil
}

func exitWithHelp(s string, rc int) {
	help := []byte(`
Usage:
	%{app} <src-dir> <dest-dir> [options]
Function: 
	Recursivly walk <src-dir> creating <dest-dir> with the same directory structure.
	Convert all '.jpg' and '.png' files to thumbnails in the <dest-dir>.
		All thumbnails are created as '.jpg' files.

	<src-dir>: is the root directory with the original pictures in it.
	<dest-dir>: is the root of the directory containing the thumbnails.

Options:
	size=n: This is the size of the thumbnail width or height depending on the originals aspect ratio.
	Default = 200. Min = 10. Max = 1000.

	If height > width then size will be the width. Aspect ratio is maintained.
	If width > height then size will be the height. Aspect ratio is maintained.
	
	All thumbnails will be rotated according to the EXIF Orientation meta data field if available.

	mask=<filename-mask>: This is the file name mask used to generate the name of the thumbnail.
	Default value is '%YYYY_%MM_%DD_%h_%m_%s_%n.%x'. This sorts file names in date time order.

	Value	Desc
	%YYYY	is a 4 digit year
	%MM	is a 2 digit month
	%DD	is a 2 digit day of month
	%h	is a 2 digit hour in 24 hour format
	%m	is a 2 digit minute
	%s	is a 2 digit second
	%n	is the name of the original file without the suffix (.jpg)
		For an image file ~/Pictures/myPic.jpg, %n is 'myPic'
	%x	is always 'jpg' which is the format of the thumbnail file.
	
	The time used is derived from the EXIF DateTimeOriginal meta data in the original image.
	If that is not available then the file name is parsed (format "20060102_150405.jpg") for a date time.
	If that fails then the file system 'modified' date time is used.
	As a last resort the current date time is used.

	noclobber: Will not overrwrite existing thumbnail files with the same file name.
	Default = clobber

	verbose: Will echo the conversion of each file to the console in addition to errors.
	Default = not verbose

	help: Echo this help text and exit the application with return code 0

Thanks:
	https://github.com/rwcarlsen/goexif (rwcarlsen) for the excelelent EXIF library. 
	https://pkg.go.dev/github.com/liujiawm/graphics-go (liujiawm) for porting the graphics library from the original Google code.
`)
	if s != "" {
		fmt.Printf("Error: %s", s)
	}
	fmt.Println(strings.ReplaceAll(string(help), "%{app}", os.Args[0]))
	os.Exit(rc)
}
