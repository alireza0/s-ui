package util

func ShadowsocksClientConfigKey(method string) string {
	switch method {
	case "2022-blake3-aes-128-gcm":
		return "shadowsocks16"
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		return "shadowsocks32"
	default:
		return "shadowsocks"
	}
}
