package util

import "encoding/base64"

// Function to return decoded bytes if a string is Base64 encoded
func StrOrBase64Encoded(str string) string {
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err == nil {
		return string(decoded)
	}
	return str
}

func B64StrToByte(str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(str)
}

func ByteToB64Str(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
