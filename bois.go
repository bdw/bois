package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// some constants
const ItemLength = 18
const CreateAttempts = 10
const FanOut = 3
const SourceFileName = "source.jpeg"
const MetaFileName = "metadata.txt"

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

func getSource(filename string) string {
	return path.Join(path.Dir(filename), SourceFileName)
}

func loadImage(filename string) (image.Image, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func makeImage(filename string, file *os.File) error {
	operation, err := ParseOperation(path.Base(filename))
	if err != nil {
		// aggressively remove such files
		os.Remove(filename)
		return err
	}
	source, err := loadImage(getSource(filename))
	if err != nil {
		// should not happen, but it can
		return err
	}
	img, err := operation.T.Apply(source)
	if err != nil {
		return err
	}
	// ignore errors here
	file.Truncate(0)
	if err = operation.F.Save(img, file); err != nil {
		return err
	}
	// and here
	file.Seek(0, 0)
	return nil
}

func touchFile(sourcePath, formatName string) (string, error) {
	operation, err := ParseOperation(formatName)
	if err != nil {
		return "", err
	}
	baseName := fmt.Sprintf("%s%s", operation.T.Name(), operation.F.Suffix())
	fullPath := path.Join(path.Dir(sourcePath), baseName)
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	return fullPath, nil
}

func saveMetadata(sourcePath, metaData string) (string, error) {
	fileName := path.Join(path.Dir(sourcePath), MetaFileName)
	file, err := os.OpenFile(fileName,  os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.WriteString(metaData); err != nil {
		return "", err
	}
	return fileName, nil
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
		fmt.Fprintln(w, "OK")
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
			http.NotFound(w, r) // bluff
		} else {
			http.ServeContent(w, r, info.Name(), time.Now(), file)
		}
	} else {
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	}
}

func (s Handler) Post(w http.ResponseWriter, r *http.Request) {
	fullPath := s.filePath(r.URL)
	if path.Base(fullPath) != SourceFileName {
		http.Error(w, "Bad Request", 400)
		return
	}
	if _, err := os.Stat(fullPath);  err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Internal Server Error", 500)
		}
		return
	}
	if format := r.FormValue("format"); format != "" {
		filename, err := touchFile(fullPath, format)
		if err != nil {
			http.Error(w, "Bad Request", 400)
			log.Println(err)
		} else {
			http.Redirect(w, r, s.urlPath(filename), 301)
		}
		return
	}

	if metaData := r.FormValue("metadata"); metaData != "" {
		filename, err := saveMetadata(fullPath, metaData)
		if err != nil {
			http.Error(w, "Internal Server Error", 500)
		} else {
			http.Redirect(w, r, s.urlPath(filename), 301)
		}
		return
	}
	http.Error(w, "Bad Request", 400)
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

func imageDirname() string {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(pwd, "images")
}

func mustMakeDir(dirName string) {
	if err := os.MkdirAll(dirName, 0755); err != nil {
		panic(err)
	}
}

func main() {
	dirName := imageDirname()
	mustMakeDir(dirName)
	log.Fatal(http.ListenAndServe(":8080", &Handler{dirName}))
}
