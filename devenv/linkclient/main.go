// Command linkclient turns a share link into an Xray client config (SOCKS on
// 127.0.0.1:10808) through upstream's own link parser, like a link-importing client.
//
//	go run ./devenv/linkclient 'hysteria2://…' > client.json && xray run -c client.json
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: linkclient <share link>")
		os.Exit(2)
	}
	res, err := link.ParseLink(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse link:", err)
		os.Exit(1)
	}
	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": 10808, "protocol": "socks",
			"settings": map[string]any{"udp": false},
		}},
		"outbounds": []any{res.Outbound},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
