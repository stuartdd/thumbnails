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

	"github.com/liujiawm/graphics-go/graphics"
	"github.com/rwcarlsen/goexif/exif"
)

type Picture struct {
	source      string
	orientation int
	err         error
}

func NewPicture(source string) *Picture {
	f, err := os.Open(source)
	if err != nil {
		return &Picture{source: source, orientation: 1, err: err}
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return &Picture{source: source, orientation: 1, err: err}
	}
	i, err := x.Get(exif.Orientation)
	if err != nil {
		return &Picture{source: source, orientation: 1, err: err}
	}
	iv, err := i.Int(0)
	if err != nil {
		return &Picture{source: source, orientation: 1, err: err}
	}
	return &Picture{source: source, orientation: iv, err: nil}
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
	sizeInt := 200
	if len(os.Args) > 3 {
		size := os.Args[3]
		sizeI, err := strconv.Atoi(size)
		if err != nil {
			log.Fatalf("Size argument '%s' is invalid %s", os.Args[3], err.Error())
		}
		if sizeI < 30 {
			log.Fatalf("Size argument '%s' is less than 30", os.Args[3])
		}
		sizeInt = sizeI
	}

	filepath.Walk(srcPath, func(inPath string, info fs.FileInfo, errIn error) error {
		if !info.IsDir() {
			relPath, fn := filepath.Split(inPath[len(srcPath):])
			relPath = filepath.Clean(relPath)
			outPath := filepath.Clean(fmt.Sprintf("%s%s", dstPath, relPath))
			_, err = os.Stat(outPath)
			if err != nil {
				os.MkdirAll(outPath, os.ModePerm)
			}
			thumb(inPath, outPath, fn, sizeInt)

		}
		return nil
	})
}

func thumb(src, dir, fn string, size int) {
	pic := NewPicture(src)
	if pic.err != nil {
		logError("EXIF  :", src, pic.err)
	}

	_, f := filepath.Split(fn)
	ext := filepath.Ext(f)
	n := f[0 : len(f)-len(ext)]
	dst := fmt.Sprintf("%s%cTN_%s.jpg", dir, filepath.Separator, n)

	imagePath, err := os.Open(src)
	if err != nil {
		logError("OPEN:  ", src, err)
		return
	}
	defer imagePath.Close()
	srcImage, _, err := image.Decode(imagePath)
	if err != nil {
		logError("DECODE:", src, err)
		return
	}

	fmt.Printf("Orientation:%d in:%s: out:%s\n", pic.orientation, src, dst)

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
		logError("THUMB :", src, err)
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
				logError("ROTATE:  90:", src, rotErr)
			} else {
				dstImage = rotImage
			}
		case 3:
			rotImage := image.NewRGBA(image.Rect(0, 0, sw, sh))
			rotErr := graphics.Rotate(rotImage, dstImage, &graphics.RotateOptions{Angle: 3.14159}) // 180
			if rotErr != nil {
				logError("ROTATE: 180:", src, rotErr)
			} else {
				dstImage = rotImage
			}
		case 8:
			rotImage := image.NewRGBA(image.Rect(0, 0, sh, sw))
			rotErr := graphics.Rotate(rotImage, dstImage, &graphics.RotateOptions{Angle: 4.71239}) // 270
			if rotErr != nil {
				logError("ROTATE: 270:", src, rotErr)
			} else {
				dstImage = rotImage
			}
		}
	}

	newImage, err := os.Create(dst)
	if err != nil {
		logError("CREATE:", dst, err)
		return
	}
	defer newImage.Close()

	err = jpeg.Encode(newImage, dstImage, &jpeg.Options{jpeg.DefaultQuality})
	if err != nil {
		logError("ENCODE:", dst, err)
		return
	}
}

func logError(tag, file string, err error) {
	if err == nil {
		log.Printf("%s %s\n", tag, file)
	} else {
		log.Printf("%s %s: %s\n", tag, file, err.Error())
	}
}
