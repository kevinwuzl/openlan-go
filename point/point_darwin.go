package point

import (
	"github.com/danieldin95/openlan-go/libol"
	"github.com/danieldin95/openlan-go/main/config"
	"github.com/danieldin95/openlan-go/models"
	"strings"
)

type Point struct {
	MixPoint
	BrName string

	addr   string
	routes []*models.Route
}

func NewPoint(config *config.Point) *Point {
	p := Point{
		BrName:   config.If.Bridge,
		MixPoint: NewMixPoint(config),
	}
	return &p
}

func (p *Point) Initialize() {
	p.worker.Listener.AddAddr = p.AddAddr
	p.worker.Listener.DelAddr = p.DelAddr
	p.worker.Listener.AddRoutes = p.AddRoutes
	p.worker.Listener.DelRoutes = p.DelRoutes
	p.MixPoint.Initialize()
}

func (p *Point) Start() {
	libol.Info("Point.Start: Darwin.")
	if !p.initialize {
		p.Initialize()
	}
	p.worker.Start()
}

func (p *Point) Stop() {
	defer libol.Catch("Point.Stop")
	p.worker.Stop()
}

func (p *Point) AddAddr(ipStr string) error {
	if ipStr == "" {
		return nil
	}

	// add point-to-point
	ips := strings.SplitN(ipStr, "/", 2)
	out, err := libol.IpAddrAdd(p.IfName(), ips[0], ips[0])
	if err != nil {
		libol.Error("Point.AddAddr: %s, %s", err, out)
		return err
	}
	libol.Info("Point.AddAddr: %s", ipStr)

	// add directly route.
	out, err = libol.IpRouteAdd(p.IfName(), ipStr, "")
	if err != nil {
		libol.Error("Point.AddAddr: %s, %s", err, out)
	}
	libol.Info("Point.AddAddr: route %s via %s", ipStr, p.IfName())

	p.addr = ipStr

	return nil
}

func (p *Point) DelAddr(ipStr string) error {
	// delete directly route.
	out, err := libol.IpRouteDel(p.IfName(), ipStr, "")
	if err != nil {
		libol.Error("Point.DelAddr: %s, %s", err, out)
	}
	libol.Info("Point.DelAddr: route %s via %s", ipStr, p.IfName())

	// delete point-to-point
	ip4 := strings.SplitN(ipStr, "/", 2)[0]
	out, err = libol.IpAddrDel(p.IfName(), ip4)
	if err != nil {
		libol.Error("Point.DelAddr: %s, %s", err, out)
		return err
	}
	libol.Info("Point.DelAddr: %s", ip4)
	p.addr = ""

	return nil
}

func (p *Point) AddRoutes(routes []*models.Route) error {
	if routes == nil {
		return nil
	}

	for _, route := range routes {
		out, err := libol.IpRouteAdd(p.IfName(), route.Prefix, "")
		if err != nil {
			libol.Error("Point.AddRoutes: %s, %s", err, out)
			continue
		}
		libol.Info("Point.AddRoutes: route %s via %s", route.Prefix, p.IfName())
	}

	p.routes = routes
	return nil
}

func (p *Point) DelRoutes(routes []*models.Route) error {
	for _, route := range routes {
		out, err := libol.IpRouteDel(p.IfName(), route.Prefix, "")
		if err != nil {
			libol.Error("Point.DelRoutes: %s, %s", err, out)
			continue
		}
		libol.Info("Point.DelRoutes: route %s via %s", route.Prefix, p.IfName())
	}

	p.routes = nil

	return nil
}
