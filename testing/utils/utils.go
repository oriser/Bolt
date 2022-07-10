package utils

import (
	"math/rand"
	"time"
)

var CapitalLetters = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
var LowerLetters = []rune("abcdefghijklmnopqrstuvwxyz")
var NumberLetters = []rune("1234567890")

func init() {
	rand.Seed(time.Now().UnixNano())
}

func GenerateRandomString(letters []rune, size int) string {
	b := make([]rune, size)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
