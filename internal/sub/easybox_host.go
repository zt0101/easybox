package sub

// SubRequestHostOverride lets EasyBox map the host a subscription request came
// in on before it becomes the default node address: a request through the
// subscription's Cloudflare hostname must not hand that hostname out, or direct
// lines (REALITY, Hysteria2, …) would point clients at Cloudflare. nil = keep.
var SubRequestHostOverride func(host string) string
