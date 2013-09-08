package main

import (
	"encoding/base64"
	"crypto/rand"
	"errors"
	"path"
	"net/http"
	"net/url"
	"log"
	"strings"
	"fmt"
	"image"
	"image/jpeg"
	"os"
)

// some constants
const ItemLength = 18
const CreateAttempts = 10
const FanOut = 3
const SourceFileName = "source.jpeg"


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
	flags := os.O_RDWR|os.O_CREATE|os.O_EXCL
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
