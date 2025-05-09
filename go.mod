module s-ui

go 1.23.2

toolchain go1.24.1

require (
	github.com/gin-contrib/gzip v1.2.3
	github.com/gin-gonic/gin v1.10.0
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/robfig/cron/v3 v3.0.1
	github.com/sagernet/sing v0.6.10-0.20250505040842-ba62fee9470f
	github.com/sagernet/sing-box v1.11.10
	github.com/sagernet/sing-dns v0.4.3
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20241231184526-a9ab2273dd10
	gorm.io/driver/sqlite v1.5.7
	gorm.io/gorm v1.26.1
)

require (
	github.com/caddyserver/zerossl v0.1.3 // indirect
	github.com/metacubex/utls v1.7.0-alpha.2 // indirect
	github.com/mholt/acmez/v3 v3.1.2 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
)

require (
	github.com/ajg/form v1.5.1 // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/bytedance/sonic v1.13.2 // indirect
	github.com/bytedance/sonic/loader v0.2.4 // indirect
	github.com/caddyserver/certmagic v0.23.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/cretz/bine v0.2.0 // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/gin-contrib/sessions v1.0.3
	github.com/gin-contrib/sse v1.0.0 // indirect
	github.com/go-chi/chi/v5 v5.2.1 // indirect
	github.com/go-chi/render v1.0.3 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.26.0 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gofrs/uuid/v5 v5.3.2 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/gorilla/context v1.1.2 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/gorilla/sessions v1.4.0 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/libdns/alidns v1.0.4-libdns.v1.beta1 // indirect
	github.com/libdns/cloudflare v0.2.2-0.20250430151523-b46a2b0885f6 // indirect
	github.com/libdns/libdns v1.0.0-beta.1 // indirect; indiresct
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v1.14.24 // indirect
	github.com/mdlayher/netlink v1.7.3-0.20250113171957-fbb4dce95f42 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/metacubex/tfo-go v0.0.0-20241231083714-66613d49c422 // indirect
	github.com/miekg/dns v1.1.65 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/sagernet/bbolt v0.0.0-20231014093535-ea5cb2fe9f0a // indirect
	github.com/sagernet/cors v1.2.1 // indirect
	github.com/sagernet/fswatch v0.1.1 // indirect
	github.com/sagernet/gvisor v0.0.0-20250325023245-7a9c0f5725fb // indirect
	github.com/sagernet/netlink v0.0.0-20240612041022-b9a21c07ac6a // indirect
	github.com/sagernet/nftables v0.3.0-beta.4 // indirect
	github.com/sagernet/quic-go v0.51.0-beta.1 // indirect
	github.com/sagernet/sing-mux v0.3.2 // indirect
	github.com/sagernet/sing-quic v0.4.1 // indirect
	github.com/sagernet/sing-shadowsocks v0.2.7 // indirect
	github.com/sagernet/sing-shadowsocks2 v0.2.0 // indirect
	github.com/sagernet/sing-shadowtls v0.2.1-0.20250503051639-fcd445d33c11 // indirect
	github.com/sagernet/sing-tun v0.6.6-0.20250428031943-0686f8c4f210 // indirect
	github.com/sagernet/sing-vmess v0.2.2-0.20250503051933-9b4cf17393f8 // indirect
	github.com/sagernet/smux v1.5.34-mod.2 // indirect
	github.com/sagernet/wireguard-go v0.0.1-beta.7 // indirect
	github.com/sagernet/ws v0.0.0-20231204124109-acfe8907c854 // indirect
	github.com/shirou/gopsutil/v4 v4.25.4
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.9.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba // indirect
	golang.org/x/arch v0.16.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	golang.org/x/tools v0.32.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
)
