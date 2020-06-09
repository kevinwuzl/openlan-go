module github.com/danieldin95/openlan-go

go 1.12

require (
	github.com/akavel/rsrc v0.8.0 // indirect
	github.com/coreos/go-systemd/v22 v22.0.0
	github.com/danieldin95/lightstar v0.0.0-20200401145448-034e11afcf81
	github.com/gorilla/mux v1.7.4
	github.com/pkg/errors v0.9.1
	github.com/songgao/water v0.0.0-20190725173103-fd331bda3f4b
	github.com/stretchr/testify v1.5.1
	github.com/urfave/cli v1.22.4
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df // indirect
	github.com/xtaci/kcp-go/v5 v5.5.12
	github.com/xtaci/kcptun v0.0.0-20200520151335-912a97993e20
	github.com/xtaci/smux v1.5.14
	github.com/xtaci/tcpraw v1.2.25
	golang.org/x/crypto v0.0.0
	golang.org/x/net v0.0.0
	golang.org/x/sys v0.0.0 // indirect
	gopkg.in/yaml.v2 v2.2.2
)

replace (
	golang.org/x/crypto => github.com/golang/crypto v0.0.0-20200604202706-70a84ac30bf9
	golang.org/x/net => github.com/golang/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/sys => github.com/golang/sys v0.0.0-20190209173611-3b5209105503
)
