package service

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

const (
	latestReleaseURL = "https://api.github.com/repos/alireza0/s-ui/releases/latest"
	// GitHub allows 60 unauthenticated calls an hour per IP, shared with
	// whatever else runs on the host; a release check needs nowhere near that.
	updateCheckTTL = 6 * time.Hour
	// A failed lookup is retried sooner, but not on every page load of a
	// server that cannot reach GitHub at all.
	updateRetryTTL = 30 * time.Minute
)

// How this installation can be updated, which decides what the panel offers:
// an in-place upgrade, or instructions for the operator.
const (
	UpdateSystemd = "systemd"
	UpdateDocker  = "docker"
	UpdateManual  = "manual"
)

type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	URL       string `json:"url"`
	HasUpdate bool   `json:"hasUpdate"`
	Method    string `json:"method"`
	tag       string
}

type UpdateService struct{}

var updateCache struct {
	sync.Mutex
	info      UpdateInfo
	checkedAt time.Time
	ttl       time.Duration
}

// CheckUpdate reports the newest published release against the running
// version. The lookup is cached for every admin, and a failure answers with
// no newer version rather than an error: the panel only shows the result.
func (s *UpdateService) CheckUpdate() UpdateInfo {
	updateCache.Lock()
	defer updateCache.Unlock()
	if !updateCache.checkedAt.IsZero() && time.Since(updateCache.checkedAt) < updateCache.ttl {
		return updateCache.info
	}

	current := config.GetVersion()
	info := UpdateInfo{Current: current, Method: updateMethod()}
	tag, url, err := fetchLatestRelease()
	updateCache.ttl = updateCheckTTL
	if err != nil {
		logger.Warning("unable to check for a new release:", err)
		updateCache.ttl = updateRetryTTL
	} else {
		info.Latest = strings.TrimPrefix(tag, "v")
		info.URL = url
		info.tag = tag
		info.HasUpdate = isNewerVersion(info.Latest, current)
	}
	updateCache.info = info
	updateCache.checkedAt = time.Now()
	return info
}

func fetchLatestRelease() (string, string, error) {
	req, err := http.NewRequest("GET", latestReleaseURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", config.GetName()+"/"+config.GetVersion())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", common.NewErrorf("release check: unexpected status code %d", resp.StatusCode)
	}
	var release struct {
		TagName string `json:"tag_name"`
		HtmlURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}
	if release.TagName == "" {
		return "", "", common.NewError("release check: no tag in the answer")
	}
	return release.TagName, release.HtmlURL, nil
}

// isNewerVersion compares dotted numeric versions. Anything that does not
// parse is never newer, so a malformed tag cannot raise a false alert.
func isNewerVersion(latest, current string) bool {
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	for i := 0; i < len(l) || i < len(c); i++ {
		var lv, cv int
		if i < len(l) {
			lv = l[i]
		}
		if i < len(c) {
			cv = c[i]
		}
		if lv != cv {
			return lv > cv
		}
	}
	return false
}

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		nums[i] = n
	}
	return nums, true
}
