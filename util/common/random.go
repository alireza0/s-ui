package common

import (
	"math/rand"
	"time"
)

var (
	allSeq []rune
	rnd    = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func init() {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	for _, char := range chars {
		allSeq = append(allSeq, char)
	}
}

func Random(n int) string {
	runes := make([]rune, n)
	for i := 0; i < n; i++ {
		runes[i] = allSeq[rnd.Intn(len(allSeq))]
	}
	return string(runes)
}

func RandomInt(n int) int {
	return rnd.Intn(n)
}
