package util

import "testing"

// Gecko arrived with sing-box 1.14; only salamander was read from a link, so a
// gecko node imported with no obfuscation at all. #1328
func TestHysteria2LinkObfsTypes(t *testing.T) {
	for _, obfsType := range []string{"salamander", "gecko"} {
		t.Run(obfsType, func(t *testing.T) {
			out, _, err := GetOutbound("hysteria2://pass@example.com:443/?sni=example.com&obfs="+obfsType+"&obfs-password=secret#node", 0)
			if err != nil {
				t.Fatal(err)
			}
			obfs, _ := (*out)["obfs"].(map[string]interface{})
			if obfs["type"] != obfsType || obfs["password"] != "secret" {
				t.Fatalf("obfs = %v, want %s with the link's password", (*out)["obfs"], obfsType)
			}
		})
	}
}
