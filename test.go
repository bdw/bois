package main

import (
	"fmt"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"regexp"
)


var SuffixRegexp = regexp.MustCompile("\\.(jpe?g|png)$")
var ScaleRegexp = regexp.MustCompile("^(scale-)?(\\d+)x(\\d+)$")
var ClipRegexp = regexp.MustCompile("^clip-(\\d+)x(\\d+)$")
var CropRegexp = regexp.MustCompile("^crop-(\\d+)x(\\d+)(-x(\\d+)y(\\d+))?$")
var CutRegexp = regexp.MustCompile("^cut-(\\d+)x(\\d+)-l(\\d+)t(\\d+)(-s(\\d+)x(\\d+))?$")


func main() {
	slice := make([]byte, 18)
	rand.Read(slice)
	encoded := base64.URLEncoding.EncodeToString(slice)
	fmt.Println(encoded)
	
	foo := strings.SplitN("abcdefghijklmnop", "", 4)
	fmt.Println(foo)
	
	if parts := ScaleRegexp.FindStringSubmatch("400x600"); parts != nil {
		fmt.Println(parts, len(parts))
	}
	if parts := CropRegexp.FindStringSubmatch("crop-400x600-x0y50"); parts != nil {
		fmt.Println(len(parts), parts)
	}
	if parts := CutRegexp.FindStringSubmatch("cut-20x20-l0t50-s40x40"); parts != nil {
		fmt.Println(len(parts), parts)
	}
	var filename string = "foo-bar.jpeg"
	if suffix := SuffixRegexp.FindStringIndex(filename); suffix != nil {
		fmt.Println(filename[suffix[0]:suffix[1]])
	}
}
