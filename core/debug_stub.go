//go:build !(linux || darwin)

package core

func rusageMaxRSS() float64 {
	return -1
}
