package bois

import (
//	"errors"
//	"strings"
//	"strconv"
	"image"
	"code.google.com/p/graphics-go/graphics"
	"image/draw"
//	"image/jpeg"
//	"image/png"
)

type Operation interface {
	Apply(image.Image) (image.Image, error)
}

type Format struct {
	name, _type string
	op Operation
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

type scaleOperation struct {
	width, height int
}

func (s scaleOperation) Apply(in image.Image) (image.Image, error) {
	out := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	if err := graphics.Scale(out, in); err != nil {
		return nil, err
	} 
	return out, nil
}

type cropOperation struct {
	width, height int
	x, y int // center points, as a percentage
}

func (s cropOperation) Apply(in image.Image) (image.Image, error) {
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
	left := clip(0, scale.Dx() - s.width, centerX - (s.width / 2))
	top := clip(0, scale.Dy() - s.height, centerY - (s.width / 2))
	// cut the correct piece
	draw.Draw(out, out.Bounds(), tmp, image.Pt(left, top), draw.Src)
	return out, nil
}

type clipOperation struct {
	width, height int
}

func (s clipOperation) Apply(in image.Image) (image.Image, error) {
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

type cutOperation struct {
	width, height int
	top, left int
	scaleWidth, scaleHeight int
}

func (s cutOperation) Apply(in image.Image) (image.Image, error) {
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

