package common

import (
	crand "crypto/rand"
	"math/big"
	mrand "math/rand"
	"sync"
	"time"
)

var (
	allSeq []rune = []rune{
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	}

	fallbackRand = mrand.New(mrand.NewSource(time.Now().UnixNano()))
	fallbackMu   = sync.Mutex{}
)

func Random(n int) string {
	if n <= 0 || len(allSeq) == 0 {
		return ""
	}
	result := make([]rune, n)
	maxBig := big.NewInt(int64(len(allSeq)))
	for i := 0; i < n; i++ {
		num, err := crand.Int(crand.Reader, maxBig)
		if err != nil {
			// fallback
			fallbackMu.Lock()
			result[i] = allSeq[fallbackRand.Intn(len(allSeq))]
			fallbackMu.Unlock()
			continue
		}
		result[i] = allSeq[int(num.Int64())]
	}
	return string(result)
}

func RandomInt(n int) int {
	if n <= 0 {
		return 0
	}
	max := big.NewInt(int64(n))
	result, err := crand.Int(crand.Reader, max)
	if err != nil {
		// fallback
		fallbackMu.Lock()
		defer fallbackMu.Unlock()
		return fallbackRand.Intn(n)
	}
	return int(result.Int64())
}
