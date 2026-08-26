module github.com/webteractive/hydra

go 1.26

// Floor, not a pin: go1.26.4 and earlier carry reachable standard-library
// vulnerabilities in net/http, crypto/tls, net/url and encoding/asn1 — all of
// which self-update touches when it fetches and verifies a release over TLS.
// Setting this here makes every build honour the floor, rather than relying on
// CI's setup-go happening to resolve a patched release.
toolchain go1.26.6

require (
	github.com/spf13/cobra v1.10.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)
