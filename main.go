package main

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"os"
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

	NC_ARG   = "noclobber"
	VB_ARG   = "verbose"
	MASK_ARG = "mask="
	SIZE_ARG = "size="
)

func NewPicture(source string) *Picture {
	_, fName := filepath.Split(source)
	ext := filepath.Ext(fName)
	name := fName[0 : len(fName)-len(ext)]

	stat, err := os.Stat(source)
	if err != nil {
		return &Picture{source: source, name: name, orientation: 1, modTime: time.Now(), time: time.Now(), err: err}
	}
	modTime := stat.ModTime()
	picTime, err := timeParseStr(name)
	if err != nil {
		picTime = modTime
	}

	f, err := os.Open(source)
	if err != nil {
		return &Picture{source: source, name: name, orientation: 1, modTime: modTime, time: picTime, err: err}
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return &Picture{source: source, name: name, orientation: 1, modTime: modTime, time: picTime, err: err}
	}
	i, err := x.Get(exif.Orientation)
	if err != nil {
		return &Picture{source: source, name: name, orientation: 1, modTime: modTime, time: picTime, err: err}
	}
	iv, err := i.Int(0)
	if err != nil {
		return &Picture{source: source, name: name, orientation: 1, modTime: modTime, time: picTime, err: err}
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
	return &Picture{source: source, name: name, orientation: iv, modTime: modTime, time: t, err: nil}
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

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Not enough args [%d]. Requires srcDir destDir. Plus an optional size.\nFor example\n - %s srcdir destdir 200", len(os.Args)-1, os.Args[0])
	}
	srcPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatalf("Source path '%s' is invalid %s", os.Args[1], err.Error())
	}
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		log.Fatalf("Source path:%s", err.Error()[5:])
	}
	if !srcInfo.IsDir() {
		log.Fatalf("Source path '%s' must be a directory.", srcPath)
	}
	dstPath, err := filepath.Abs(os.Args[2])
	if err != nil {
		log.Fatalf("Destination path '%s' is invalid %s", os.Args[2], err.Error())
	}
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		log.Fatalf("Destination path:%s", err.Error()[5:])
	}
	if !dstInfo.IsDir() {
		log.Fatalf("Destination path '%s' must be a directory.", srcPath)
	}
	sizeInt, err := findIntArg(SIZE_ARG, 10, 1000, 200)
	if err != nil {
		log.Fatal(err)
	}
	fileNameMask := findStringArg(MASK_ARG, NAME_MASK)
	noClobber := findBoolArg(NC_ARG, true)
	verbose := findBoolArg(VB_ARG, true)

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

func thumb(srcFile, thumbPath, thumbNameMask string, size int, noClobber, verbose bool) {
	pic := NewPicture(srcFile)
	if pic.err != nil {
		logError("EXIF  :", srcFile, pic.err)
	}
	thumbFileName := fmt.Sprintf("%s%c%s", thumbPath, filepath.Separator, subFileName(pic.time, thumbNameMask, pic.name, "jpg"))
	if noClobber {
		_, err := os.Stat(thumbFileName)
		if err == nil {
			return
		}
	}
	imagePath, err := os.Open(srcFile)
	if err != nil {
		logError("OPEN:  ", srcFile, err)
		return
	}
	defer imagePath.Close()
	srcImage, _, err := image.Decode(imagePath)
	if err != nil {
		logError("DECODE:", srcFile, err)
		return
	}
	if verbose {
		logError("INFO  :", fmt.Sprintf("Orientation:%d in:%s: out:%s", pic.orientation, srcFile, thumbFileName), nil)
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

	dstImage := image.NewRGBA(image.Rect(0, 0, sw, sh))
	err = graphics.Thumbnail(dstImage, srcImage)
	if err != nil {
		logError("THUMB :", srcFile, err)
		return
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
				logError("ROTATE:  90:", srcFile, rotErr)
			} else {
				dstImage = rotImage
			}
		case 3:
			rotImage := image.NewRGBA(image.Rect(0, 0, sw, sh))
			rotErr := graphics.Rotate(rotImage, dstImage, &graphics.RotateOptions{Angle: 3.14159}) // 180
			if rotErr != nil {
				logError("ROTATE: 180:", srcFile, rotErr)
			} else {
				dstImage = rotImage
			}
		case 8:
			rotImage := image.NewRGBA(image.Rect(0, 0, sh, sw))
			rotErr := graphics.Rotate(rotImage, dstImage, &graphics.RotateOptions{Angle: 4.71239}) // 270
			if rotErr != nil {
				logError("ROTATE: 270:", srcFile, rotErr)
			} else {
				dstImage = rotImage
			}
		}
	}

	newImage, err := os.Create(thumbFileName)
	if err != nil {
		logError("CREATE:", thumbFileName, err)
		return
	}
	defer newImage.Close()

	err = jpeg.Encode(newImage, dstImage, &jpeg.Options{Quality: jpeg.DefaultQuality})
	if err != nil {
		logError("ENCODE:", thumbFileName, err)
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

func logError(tag, file string, err error) {
	if err == nil {
		log.Printf("%s %s\n", tag, file)
	} else {
		log.Printf("%s %s: %s\n", tag, file, err.Error())
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
