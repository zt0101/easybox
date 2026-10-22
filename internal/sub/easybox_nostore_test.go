package sub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// EasyBox serves the subscription through Cloudflare too: no subscription body
// may be cacheable, or a CDN could hand out a stale node list.
func TestSubscriptionBodiesAreNotCacheable(t *testing.T) {
	initSubDB(t)
	seedInfoEndpointSub(t, "nostore-sub", "nostore@example.com")
	for _, s := range []model.Setting{
		{Key: "subPath", Value: "/sub/"},
		{Key: "subJsonPath", Value: "/json/"},
		{Key: "subClashPath", Value: "/clash/"},
		{Key: "subJsonEnable", Value: "true"},
		{Key: "subClashEnable", Value: "true"},
	} {
		database.GetDB().Where("key = ?", s.Key).Delete(&model.Setting{})
		if err := database.GetDB().Create(&s).Error; err != nil {
			t.Fatal(err)
		}
	}
	router, err := (&Server{}).initRouter()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/sub/nostore-sub", "/json/nostore-sub", "/clash/nostore-sub", "/mihomo/nostore-sub"} {
		for _, ua := range []string{"v2rayNG/2.3.11", "mihomo/1.19.32", "SFA/1.14.3 (8; sing-box 1.14.3; language en_US)"} {
			req := httptest.NewRequest(http.MethodGet, "https://sub.example.com:2096"+path, nil)
			req.Header.Set("User-Agent", ua)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if got := resp.Header().Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
				t.Fatalf("%s (%s): status %d, Cache-Control %q", path, ua, resp.Code, got)
			}
		}
	}
}
