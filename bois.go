package main

import (
	"code.google.com/p/graphics-go/graphics"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"strconv"
	"regexp"
	"io"
	"time"
)

// some constants
const ItemLength = 18
const CreateAttempts = 10
const FanOut = 3
const SourceFileName = "source.jpeg"

// image transformations


var SuffixRegexp = regexp.MustCompile("\\.((q\\d+\\.)?jpe?g|png)$")
var ScaleRegexp = regexp.MustCompile("^(scale-)?(\\d+)x(\\d+)$")
var ClipRegexp = regexp.MustCompile("^clip-(\\d+)x(\\d+)$")
var CropRegexp = regexp.MustCompile("^crop-(\\d+)x(\\d+)(-x(\\d+)y(\\d+))?$")
var CutRegexp = regexp.MustCompile("^cut-(\\d+)x(\\d+)-t(\\d+)l(\\d+)(-s(\\d+)x(\\d+))?$")

type Transformation interface {
	Name() string
	Apply(image.Image) (image.Image, error)
}

type Format interface {
	Save(image.Image, io.Writer) error
	Suffix() string
}

type Operation struct {
	t Transformation
	f Format
}

type jpegFormat jpeg.Options

func (j jpegFormat) Save(i image.Image, f io.Writer) error {
	o := jpeg.Options(j)
	return jpeg.Encode(f, i, &o)
}

func (j jpegFormat) Suffix() string {
	return fmt.Sprintf(".%d.jpeg", jpeg.Options(j).Quality)
}

type pngFormat struct {}

func (p pngFormat) Save(i image.Image, f io.Writer) error {
	return png.Encode(f, i)
}

func (p pngFormat) Suffix() string {
	return ".png"
}

func ParseTransformation(format string) (Transformation, error) {
	if parts := ScaleRegexp.FindStringSubmatch(format); parts != nil {
		width, _ := strconv.Atoi(parts[2])
		height, _ := strconv.Atoi(parts[3])
		return scaleTransformation{width, height}, nil
	}
	if parts := ClipRegexp.FindStringSubmatch(format); parts != nil {
		width, _ := strconv.Atoi(parts[1])
		height, _ := strconv.Atoi(parts[2])
		return clipTransformation{width, height}, nil
	}
	if parts := CropRegexp.FindStringSubmatch(format); parts != nil {
		width, _ := strconv.Atoi(parts[1])
		height, _ := strconv.Atoi(parts[2])
		x, y := 50, 50
		if len(parts[3]) > 0 {
			x, _ = strconv.Atoi(parts[4])
			y, _ = strconv.Atoi(parts[5])
		}
		return cropTransformation{width, height, x, y}, nil
	}
	if parts := CutRegexp.FindStringSubmatch(format); parts != nil {
		width, _ := strconv.Atoi(parts[1])
		height, _ := strconv.Atoi(parts[2])
		left, _ := strconv.Atoi(parts[3])
		top, _ := strconv.Atoi(parts[4])
		sX, sY := width, height
		if len(parts[5]) > 0 {
			sX, _ = strconv.Atoi(parts[6])
			sY, _ = strconv.Atoi(parts[7])
		}
		return cutTransformation{width, height, left, top, sX, sY}, nil
	}
	return nil, errors.New("cannot parse format")
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clip(minimum, maximum, vl int) int {
	return max(minimum, min(maximum, vl))
}

type scaleTransformation struct {
	width, height int
}

func (s scaleTransformation) Name() string {
	return fmt.Sprintf("scale-%dx%d", s.width, s.height)
}

func (s scaleTransformation) Apply(in image.Image) (image.Image, error) {
	out := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	if err := graphics.Scale(out, in); err != nil {
		return nil, err
	}
	return out, nil
}

type cropTransformation struct {
	width, height int
	x, y          int // center points, as a percentage
}

func (s cropTransformation) Name() string {
	return fmt.Sprintf("crop-%dx%d-x%dy%d", s.width, s.height, s.x, s.y)
}

func (s cropTransformation) Apply(in image.Image) (image.Image, error) {
	// crop first scales to the maximum size
	wOut := (s.height * in.Bounds().Dx()) / in.Bounds().Dy()
	hOut := (s.width * in.Bounds().Dy()) / in.Bounds().Dx()
	var scale image.Rectangle
	if wOut > s.width {
		// crop horizontal
		scale = image.Rect(0, 0, wOut, s.height)
	} else {
		// crop vertical
		scale = image.Rect(0, 0, s.width, hOut)
	}
	tmp := image.NewRGBA(scale)
	if err := graphics.Scale(tmp, in); err != nil {
		return nil, err
	}
	// output image, always exactly the requested bounds
	out := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	// find our 'cutting rectangle
	centerX := (s.x * scale.Dx()) / 100
	centerY := (s.y * scale.Dy()) / 100
	// clipped points
	left := clip(0, scale.Dx()-s.width, centerX-(s.width/2))
	top := clip(0, scale.Dy()-s.height, centerY-(s.width/2))
	// cut the correct piece
	draw.Draw(out, out.Bounds(), tmp, image.Pt(left, top), draw.Src)
	return out, nil
}

type clipTransformation struct {
	width, height int
}

func (s clipTransformation) Name() string {
	return fmt.Sprintf("clip-%dx%d", s.width, s.height)
}

func (s clipTransformation) Apply(in image.Image) (image.Image, error) {
	// clip keeps aspect ratio intact, thus wOut / hOut = wIn / hIn
	// thous wOut = wIn * hOut / hIn
	wOut := (s.height * in.Bounds().Dx()) / in.Bounds().Dy()
	hOut := (s.width * in.Bounds().Dy()) / in.Bounds().Dx()
	var scale image.Rectangle
	if wOut <= s.width {
		// clip vertical
		scale.Max.X = wOut
		scale.Max.Y = s.height
	} else {
		scale.Max.X = s.width
		scale.Max.Y = hOut
	}
	out := image.NewRGBA(scale)
	if err := graphics.Scale(out, in); err != nil {
		return nil, err
	}
	return out, nil
}

type cutTransformation struct {
	width, height           int
	left, top               int
	scaleWidth, scaleHeight int
}

func (s cutTransformation) Name() string {
	return fmt.Sprintf("cut-%dx%d-t%dl%d-s%dx%d", s.width, s.height, s.top, s.left, s.scaleWidth, s.scaleHeight)
}

func (s cutTransformation) Apply(in image.Image) (image.Image, error) {
	tmp := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	draw.Draw(tmp, tmp.Bounds(), in, image.Pt(s.top, s.left), draw.Src)
	if s.width == s.scaleWidth && s.height == s.scaleHeight {
		return tmp, nil
	}
	out := image.NewRGBA(image.Rect(0, 0, s.scaleWidth, s.scaleHeight))
	if err := graphics.Scale(out, tmp); err != nil {
		return nil, err
	}
	return out, nil
}



type Handler struct {
	rootDir string
}

func (s Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.Get(w, r)
	case "POST":
		s.Post(w, r)
	case "PUT":
		s.Put(w, r)
	case "DELETE":
		s.Delete(w, r)
	default:
		http.Error(w, "Method Not Allowed", 405)
	}
}

func randomString() string {
	slice := make([]byte, ItemLength)
	rand.Read(slice)
	return base64.URLEncoding.EncodeToString(slice)
}

func dirName(token string) string {
	// really happy this works, by the way
	parts := strings.SplitN(token, "", FanOut+1)
	// now here i was, almost making an argument how golang isn't elegant
	return path.Join(parts...)
}

func (s Handler) filePath(u *url.URL) string {
	return path.Join(s.rootDir, u.Path)
}

func (s Handler) urlPath(fullPath string) string {
	return strings.Replace(fullPath, s.rootDir, "", 1)
}

func (s Handler) createFile() (*os.File, string, error) {
	flags := os.O_RDWR | os.O_CREATE | os.O_EXCL
	for i := 0; i < CreateAttempts; i++ {
		dir := path.Join(s.rootDir, dirName(randomString()))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, "", err
		}
		filename := path.Join(dir, SourceFileName)
		file, err := os.OpenFile(filename, flags, 0644)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return nil, "", err
		}
		// return with a file and its name
		return file, filename, nil
	}
	return nil, "", errors.New("Exceeded attempts to create file")
}

func makeImage(filename string, file *os.File) error {
	return nil
}


// Handle an upload request
func (s Handler) Put(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, "No data sent", 400)
		return
	}
	img, _, err := image.Decode(r.Body)
	if err != nil {
		http.Error(w, "Could not decode image", 400)
		return
	}
	file, filename, err := s.createFile()
	if err != nil {
		http.Error(w, "Could not create file", 500)
		log.Print(err)
		return
	}
	defer file.Close()
	if err = jpeg.Encode(file, img, nil); err != nil {
		http.Error(w, "Could not save image", 500)
		log.Print(err)
	} else {
		// great success!
		http.Redirect(w, r, s.urlPath(filename), 301)
	}
}

// Handle a GET request
func (s Handler) Get(w http.ResponseWriter, r *http.Request) {
	fullPath := s.filePath(r.URL)
	file, err := os.OpenFile(fullPath, os.O_RDWR, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Internal Server Error", 500)
			log.Print(err)
		}
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		log.Print(err)
		return
	} else if info.IsDir() {
		http.Error(w, "Forbidden", 403)
	} else if info.Size() == 0 {
		if err = makeImage(fullPath, file); err != nil {
			os.Remove(fullPath) // don't try again
			http.NotFound(w, r) // bluff
		} else {
			http.ServeContent(w, r, info.Name(), time.Now(), file)
		}
	} else {
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	}
}

func (s Handler) Post(w http.ResponseWriter, r *http.Request) {

}

func (s Handler) Delete(w http.ResponseWriter, r *http.Request) {
	fullPath := s.filePath(r.URL)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Internal Server Error", 500)
		}
		return
	}
	if info.IsDir() {
		http.Error(w, "Forbidden", 403)
	} else if info.Name() == SourceFileName {
		if err = os.RemoveAll(path.Dir(fullPath)); err != nil {
			http.Error(w, "Internal Server Error", 500)
		} else {
			fmt.Fprintln(w, "OK")
		}
	} else {
		os.Remove(fullPath)
	}
}

func main() {
	fmt.Println("OH HAI")
}
