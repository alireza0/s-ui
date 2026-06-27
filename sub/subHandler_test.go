package sub

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAddHeadersPreservesSubscriptionHeadersAndAddsContentDisposition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	handler := &SubHandler{}
	handler.addHeaders(context, []string{
		"upload=1; download=2; total=3; expire=4",
		"12",
		"phj233_vpn",
	})

	headers := recorder.Header()
	tests := map[string]string{
		"Subscription-Userinfo":   "upload=1; download=2; total=3; expire=4",
		"Profile-Update-Interval": "12",
		"Profile-Title":           "phj233_vpn",
		"Content-Disposition":     "attachment; filename=\"phj233_vpn\"; filename*=UTF-8''phj233_vpn",
	}

	for key, want := range tests {
		if got := headers.Get(key); got != want {
			t.Fatalf("header %s = %q, want %q", key, got, want)
		}
	}
}

func TestContentDispositionHeaderUsesSubscriptionNameWithoutExtension(t *testing.T) {
	got := contentDispositionHeader("phj233_vpn")
	want := "attachment; filename=\"phj233_vpn\"; filename*=UTF-8''phj233_vpn"

	if got != want {
		t.Fatalf("contentDispositionHeader() = %q, want %q", got, want)
	}
}

func TestContentDispositionHeaderEscapesUTF8Name(t *testing.T) {
	got := contentDispositionHeader("蓝胖云 LanPangYun")
	want := "attachment; filename=\"LanPangYun\"; filename*=UTF-8''%E8%93%9D%E8%83%96%E4%BA%91%20LanPangYun"

	if got != want {
		t.Fatalf("contentDispositionHeader() = %q, want %q", got, want)
	}
}

func TestContentDispositionHeaderFallsBackWhenNameIsEmpty(t *testing.T) {
	got := contentDispositionHeader(" ")
	want := "attachment; filename=\"subscription\"; filename*=UTF-8''subscription"

	if got != want {
		t.Fatalf("contentDispositionHeader() = %q, want %q", got, want)
	}
}
