package main

import (
	"fmt"
	"crypto/rand"
	"encoding/base64"
	"strings"
)

func main() {
	slice := make([]byte, 18)
	rand.Read(slice)
	encoded := base64.URLEncoding.EncodeToString(slice)
	fmt.Println(encoded)
	
	foo := strings.SplitN("abcdefghijklmnop", "", 4)
	fmt.Println(foo)
}
