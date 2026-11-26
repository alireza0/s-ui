package nodeagent

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime/debug"
	"sync"
)

var SingBoxVersion = sync.OnceValue(func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/sagernet/sing-box" {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return ""
})

// /proc/self/exe identifies the running inode even after the file is replaced.
var ExecutableDigest = sync.OnceValue(func() string {
	file, err := os.Open("/proc/self/exe")
	if err != nil {
		return ""
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return ""
	}
	return hex.EncodeToString(sum.Sum(nil))
})
