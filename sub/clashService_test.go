package sub

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildProxyGroupsExpandsAllWithoutDefaultGroups(t *testing.T) {
	output := clashConfig(t, `proxy-groups:
  - name: Manual
    type: select
    proxies:
      - all
`)

	if err := buildProxyGroups(output, []string{"HK-1", "US-1"}, true, true); err != nil {
		t.Fatal(err)
	}

	groups := proxyGroupList(output["proxy-groups"])
	if len(groups) != 1 || groups[0]["name"] != "Manual" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
	if got := proxyNames(groups[0]["proxies"]); !reflect.DeepEqual(got, []string{"HK-1", "US-1"}) {
		t.Fatalf("Manual proxies = %#v", got)
	}
}

func TestBuildProxyGroupsMergesDefaultsAndFilters(t *testing.T) {
	output := clashConfig(t, `proxy-groups:
  - name: Proxy
    proxies:
      - AI-Proxy
  - name: HK-Group
    type: select
    filter: "HK|HongKong"
`)

	if err := buildProxyGroups(output, []string{"HK-1", "US-1"}, false, false); err != nil {
		t.Fatal(err)
	}

	groups := proxyGroupList(output["proxy-groups"])
	if len(groups) != 3 {
		t.Fatalf("group count = %d, want 3: %#v", len(groups), groups)
	}
	if got := proxyNames(groups[0]["proxies"]); !reflect.DeepEqual(got, []string{"Auto", "HK-1", "US-1", "AI-Proxy"}) {
		t.Fatalf("Proxy proxies = %#v", got)
	}
	if got := proxyNames(groups[2]["proxies"]); !reflect.DeepEqual(got, []string{"HK-1"}) {
		t.Fatalf("HK-Group proxies = %#v", got)
	}
}

func clashConfig(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var output map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatal(err)
	}
	return output
}
