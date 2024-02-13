package common

import "math/rand"

var allSeq [62]rune

func Random(n int) string {
	runes := make([]rune, n)
	for i := 0; i < n; i++ {
		runes[i] = allSeq[rand.Intn(len(allSeq))]
	}
	return string(runes)
}
