package easybox

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Line names follow <region>-<protocol>-<transport>[-CDN|-Tunnel], e.g.
// HK-VLESS-Reality or HK-VLESS-WS-CDN, so lines from several servers and
// protocols stay apart in a client. A name the admin types wins; preferred
// addresses append "-<remark>" to whatever the line is called.

const (
	regionKey     = "regionCode"
	maxLineName   = 64
	regionTimeout = 5 * time.Second
)

var regionPattern = regexp.MustCompile(`^[A-Z0-9]{2,8}$`)

// RegionView is the region code line names start with ("" = unknown, names
// then start with the protocol).
type RegionView struct {
	Code string `json:"code" example:"HK"`
}

type RegionRequest struct {
	Code string `json:"code" form:"code" example:"HK"`
}

// region returns the stored code, detecting and storing it the first time.
func (s *Service) region() string {
	if v, _, err := getSetting(regionKey); err == nil && v != "" {
		return v
	}
	lookup := s.regionLookup
	if lookup == nil {
		lookup = detectRegion
	}
	code := strings.ToUpper(strings.TrimSpace(lookup()))
	if !regionPattern.MatchString(code) {
		return ""
	}
	_ = setSetting(regionKey, code)
	return code
}

func (s *Service) GetRegion() RegionView {
	return RegionView{Code: s.region()}
}

// SetRegion stores the admin's code; an empty code detects it again.
func (s *Service) SetRegion(req RegionRequest) (RegionView, error) {
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code != "" && !regionPattern.MatchString(code) {
		return RegionView{}, errors.New("a region code is 2-8 letters or digits, e.g. HK, JP, US")
	}
	if err := setSetting(regionKey, code); err != nil {
		return RegionView{}, err
	}
	auditObject("region.set", "region", 0, nil)
	return RegionView{Code: s.region()}, nil
}

// detectRegion asks where the server's IP is: Cloudflare's trace first (its
// loc= is the country of the requesting IP), then ipinfo.io.
func detectRegion() string {
	client := &http.Client{Timeout: regionTimeout}
	get := func(url string) string {
		ctx, cancel := context.WithTimeout(context.Background(), regionTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return ""
		}
		resp, err := client.Do(req)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return ""
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return string(body)
	}
	sc := bufio.NewScanner(strings.NewReader(get("https://www.cloudflare.com/cdn-cgi/trace")))
	for sc.Scan() {
		if loc, ok := strings.CutPrefix(sc.Text(), "loc="); ok && loc != "XX" {
			return loc
		}
	}
	return strings.TrimSpace(get("https://ipinfo.io/country"))
}

// defaultName is the rule-made name of a new line, numbered past names in use.
func (s *Service) defaultName(tpl lineTemplate) (string, error) {
	base := tpl.nameStyle
	if r := s.region(); r != "" {
		base = r + "-" + base
	}
	used, err := lineNamesInUse(0)
	if err != nil {
		return "", err
	}
	if !used[strings.ToLower(base)] {
		return base, nil
	}
	for n := 2; ; n++ {
		if name := base + "-" + strconv.Itoa(n); !used[strings.ToLower(name)] {
			return name, nil
		}
	}
}

// lineNamesInUse is every line name (lower-cased) except line except's own.
func lineNamesInUse(except int) (map[string]bool, error) {
	var names []string
	if err := database.GetDB().Model(&model.EasyboxEndpoint{}).
		Where("id <> ? AND name <> ''", except).Pluck("name", &names).Error; err != nil {
		return nil, err
	}
	used := make(map[string]bool, len(names))
	for _, n := range names {
		used[strings.ToLower(n)] = true
	}
	return used, nil
}

// RenameEndpoint renames a line and the subscription node it serves.
func (s *Service) RenameEndpoint(id int, name string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return errors.New("enter a name")
	case utf8.RuneCountInString(name) > maxLineName:
		return fmt.Errorf("a line name is at most %d characters", maxLineName)
	case strings.ContainsAny(name, "#{}") || strings.IndexFunc(name, unicode.IsControl) >= 0:
		return errors.New("a line name may not contain # { } or control characters")
	}
	row, err := loadEndpoint(id)
	if err != nil {
		return err
	}
	used, err := lineNamesInUse(id)
	if err != nil {
		return err
	}
	if used[strings.ToLower(name)] {
		return fmt.Errorf("another line is already called %q", name)
	}
	db := database.GetDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.EasyboxEndpoint{}).Where("id = ?", id).Update("name", name).Error; err != nil {
			return err
		}
		if row.InboundId == 0 {
			return nil
		}
		// The remark names the subscription node ({{INBOUND}}); Xray never reads it.
		return tx.Model(&model.Inbound{}).Where("id = ?", row.InboundId).Update("remark", name).Error
	})
	audit("endpoint.rename", id, err)
	return err
}
