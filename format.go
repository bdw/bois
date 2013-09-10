package main

import (
	"code.google.com/p/graphics-go/graphics"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"regexp"
	"strconv"
	"io"
)

const DefaultQuality = 80

type Transformation interface {
	Name() string
	Apply(image.Image) (image.Image, error)
}

type Format interface {
	Save(image.Image, io.Writer) error
	Suffix() string
}

type Operation struct {
	T Transformation
	F Format
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


var SuffixRegexp = regexp.MustCompile("\\.((q(\\d+)\\.)?jpe?g|png)$")
var ScaleRegexp = regexp.MustCompile("^(scale-)?(\\d+)x(\\d+)$")
var ClipRegexp = regexp.MustCompile("^clip-(\\d+)x(\\d+)$")
var CropRegexp = regexp.MustCompile("^crop-(\\d+)x(\\d+)(-x(\\d+)y(\\d+))?$")
var CutRegexp = regexp.MustCompile("^cut-(\\d+)x(\\d+)-t(\\d+)l(\\d+)(-s(\\d+)x(\\d+))?$")



func ParseOperation (format string) (*Operation, error) {
	var tr Transformation
	var ft Format
	if parts := SuffixRegexp.FindStringSubmatch(format); parts != nil  {
		if len(parts[3]) > 0 { // we have a number, must be a jpeg
			quality, _ := strconv.Atoi(parts[3]) 
			ft  = jpegFormat(jpeg.Options{Quality: clip(0, 100, quality)})
		} else if parts[1] == "jpeg" {
			ft = jpegFormat(jpeg.Options{Quality: DefaultQuality})
		} else {
			ft = pngFormat{}
		}
		format = format[0:len(format)-len(parts[0])]
	} else {
		ft = jpegFormat(jpeg.Options{Quality: DefaultQuality})
	}
	tr, err := ParseTransformation(format)
	if err != nil {
		return nil, err
	}
	return &Operation{T: tr, F: ft}, nil
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
	top := clip(0, scale.Dy()-s.height, centerY-(s.height/2))
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

