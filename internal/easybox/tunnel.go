package easybox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

// The tunnel line is the no-domain backup: VLESS + WebSocket on 127.0.0.1,
// reached through a Cloudflare quick tunnel (https://<random>.trycloudflare.com).
// Measured from China on 2026-10-22: it works, preferred Cloudflare IPs make it
// fast and stable, the bare hostname is slow. The hostname changes whenever
// cloudflared restarts, so cloudflared runs as its own transient systemd unit:
// panel restarts, updates and certificate renewals leave it (and the hostname)
// alone; only a reboot or a crash brings a new one, which tunnelWatchOnce picks
// up and puts into the subscription.

const (
	tunnelUnit        = "easybox-tunnel"
	tunnelMetricsAddr = "127.0.0.1:20241"
	tunnelWatchEvery  = time.Minute
)

// tunnelWait is how long a new tunnel may take to report a hostname and a
// ready connection to Cloudflare.
var tunnelWait = 60 * time.Second

var (
	errTunnelBinary  = errors.New("this build ships no cloudflared for the platform")
	errTunnelSystemd = errors.New("the tunnel line needs systemd")
	errTunnelExists  = errors.New("only one tunnel line: delete the existing one first")
)

// tunnelEnvironment is everything the tunnel line touches outside the panel;
// tests replace it.
type tunnelEnvironment struct {
	binary   func() string // "" when no cloudflared ships for this platform
	systemd  func() bool
	start    func(binary string, port int) error
	stop     func() error
	active   func() bool
	hostname func() (host string, ready int, err error)
}

var tunnelEnv = systemTunnel()

func systemTunnel() tunnelEnvironment {
	run := func(name string, args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	stop := func() error {
		err := run("systemctl", "stop", tunnelUnit)
		_ = run("systemctl", "reset-failed", tunnelUnit)
		return err
	}
	return tunnelEnvironment{
		binary: func() string {
			if runtime.GOOS != "linux" {
				return ""
			}
			p, err := filepath.Abs(filepath.Join(config.GetBinFolderPath(), "cloudflared"))
			if err != nil {
				return ""
			}
			if st, err := os.Stat(p); err != nil || st.Mode()&0o111 == 0 {
				return ""
			}
			return p
		},
		systemd: func() bool {
			_, errRun := exec.LookPath("systemd-run")
			_, errCtl := exec.LookPath("systemctl")
			st, err := os.Stat("/run/systemd/system")
			return errRun == nil && errCtl == nil && err == nil && st.IsDir()
		},
		start: func(binary string, port int) error {
			_ = stop()
			// Its own unit, outside the panel's sandbox and lifetime, as an
			// unprivileged throwaway user that can write nowhere.
			return run("systemd-run", "--unit", tunnelUnit,
				"--property=Restart=always", "--property=RestartSec=5",
				"--property=DynamicUser=yes", "--property=NoNewPrivileges=yes",
				"--property=ProtectSystem=strict", "--property=ProtectHome=yes",
				"--property=PrivateTmp=yes",
				binary, "tunnel", "--no-autoupdate", "--metrics", tunnelMetricsAddr,
				"--url", "http://127.0.0.1:"+strconv.Itoa(port))
		},
		stop: stop,
		active: func() bool {
			return run("systemctl", "is-active", "--quiet", tunnelUnit) == nil
		},
		hostname: func() (string, int, error) {
			var q struct {
				Hostname string `json:"hostname"`
			}
			if err := getJSON("http://"+tunnelMetricsAddr+"/quicktunnel", &q); err != nil {
				return "", 0, err
			}
			var r struct {
				ReadyConnections int `json:"readyConnections"`
			}
			_ = getJSON("http://"+tunnelMetricsAddr+"/ready", &r) // 503 until a connection is up
			return q.Hostname, r.ReadyConnections, nil
		},
	}
}

func getJSON(url string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(out)
}

// tunnelLine is the provisioned tunnel line; a failed create (no inbound) is
// not one.
func tunnelLine() (*model.EasyboxEndpoint, bool, error) {
	var rows []model.EasyboxEndpoint
	if err := database.GetDB().Where("template = ? AND inbound_id <> 0", TemplateVlessTunnel).Order("id").Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	return &rows[0], true, nil
}

func tunnelAvailable() string {
	switch {
	case tunnelEnv.binary() == "":
		return errTunnelBinary.Error()
	case !tunnelEnv.systemd():
		return errTunnelSystemd.Error()
	}
	if _, ok, err := tunnelLine(); err != nil || ok {
		return errTunnelExists.Error()
	}
	return ""
}

// startTunnel (re)starts the unit for a local port and waits until Cloudflare
// hands out a hostname and a connection is ready.
func startTunnel(port int) (string, error) {
	binary := tunnelEnv.binary()
	if binary == "" {
		return "", errTunnelBinary
	}
	if err := tunnelEnv.start(binary, port); err != nil {
		return "", err
	}
	deadline := time.Now().Add(tunnelWait)
	var lastErr error
	for time.Now().Before(deadline) {
		host, ready, err := tunnelEnv.hostname()
		if err == nil && host != "" && ready > 0 {
			return host, nil
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	if lastErr == nil {
		lastErr = errors.New("no ready connection to Cloudflare")
	}
	return "", fmt.Errorf("Cloudflare quick tunnel did not come up in %s: %w", tunnelWait, lastErr)
}

// BuildVlessTunnelInbound is VLESS + WebSocket without TLS on 127.0.0.1:
// cloudflared is its only client and Cloudflare terminates TLS. WebSocket is
// what the 2026-10-22 test used and every client supports, sing-box included.
func BuildVlessTunnelInbound(name string, id Identity) (*model.Inbound, error) {
	settings, err := json.Marshal(vlessSettings{
		Clients:    []vlessClient{{ID: id.UUID, Email: id.Email, Enable: true, SubID: id.SubID}},
		Decryption: "none",
		Encryption: "none",
		Fallbacks:  []any{},
	})
	if err != nil {
		return nil, err
	}
	stream, err := json.Marshal(map[string]any{
		"network": "ws",
		"wsSettings": map[string]any{
			"acceptProxyProtocol": false, "path": "/" + random.NumLower(cdnPathLength), "host": "",
			"headers": map[string]any{}, "heartbeatPeriod": 0,
		},
		"security": "none",
	})
	if err != nil {
		return nil, err
	}
	return &model.Inbound{
		Remark: name, Enable: true, Listen: "127.0.0.1", Protocol: model.VLESS,
		Settings: string(settings), StreamSettings: string(stream), Sniffing: tcpSniffing,
	}, nil
}

// tunnelWatchOnce keeps the unit in step with the tunnel line: it starts it
// after a reboot, stops it when the line is gone, and moves the subscription to
// a new hostname after cloudflared restarted.
func (s *Service) tunnelWatchOnce() error {
	row, ok, err := tunnelLine()
	if err != nil {
		return err
	}
	if !ok || row.InboundId == 0 {
		if tunnelEnv.active() {
			return tunnelEnv.stop()
		}
		return nil
	}
	if !tunnelEnv.active() {
		ib, err := s.inbounds.GetInbound(row.InboundId)
		if err != nil {
			return err
		}
		if binary := tunnelEnv.binary(); binary != "" {
			if err := tunnelEnv.start(binary, ib.Port); err != nil {
				return err
			}
		}
		return nil // the next round reads the new hostname
	}
	host, _, err := tunnelEnv.hostname()
	if err != nil || host == "" || host == row.CdnHost {
		return err
	}
	old := row.CdnHost
	row.CdnHost = host
	if err := database.GetDB().Save(row).Error; err != nil {
		return err
	}
	err = syncCDNHosts()
	logger.Infof("easybox: tunnel hostname changed from %s to %s; devices must refresh the subscription", old, host)
	auditObject("tunnel.hostname", "endpoint", row.Id, err)
	return err
}

// StartTunnelWatch runs tunnelWatchOnce in the background for the panel's
// lifetime.
func StartTunnelWatch() {
	svc := NewService()
	go func() {
		time.Sleep(15 * time.Second) // let Xray come up first
		for {
			if err := svc.tunnelWatchOnce(); err != nil {
				logger.Warning("easybox: tunnel watch:", err)
			}
			time.Sleep(tunnelWatchEvery)
		}
	}()
}
