package bois

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"path"
	"fmt"
)

const ItemLength = 18
const FanOut = 3 

func randomString() string {
	slice := make([]byte, ItemLength)
	rand.Read(slice)
	return base64.URLEncoding.EncodeToString(slice)
}

func dirName(token string) string {
	// really happy this works, by the way
	parts := strings.SplitN(token, "", FanOut + 1)
	// now here i was, almost making an argument how golang isn't elegant
	return path.Join(parts...)
}


