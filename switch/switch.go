package _switch

import (
	"crypto/tls"
	"github.com/danieldin95/openlan-go/libol"
	"github.com/danieldin95/openlan-go/main/config"
	"github.com/danieldin95/openlan-go/models"
	"github.com/danieldin95/openlan-go/network"
	"github.com/danieldin95/openlan-go/switch/app"
	"github.com/danieldin95/openlan-go/switch/ctrls"
	"github.com/danieldin95/openlan-go/switch/storage"
	"strings"
	"sync"
	"time"
)

type Apps struct {
	Auth     *app.PointAuth
	Request  *app.WithRequest
	Neighbor *app.Neighbors
	OnLines  *app.Online
}

type Hook func(client libol.SocketClient, frame *libol.FrameMessage) error

type Switch struct {
	Conf config.Switch
	Apps Apps
	Fire FireWall

	hooks      []Hook
	http       *Http
	server     libol.SocketServer
	bridge     map[string]network.Bridger
	worker     map[string]*Worker
	lock       sync.RWMutex
	uuid       string
	newTime    int64
	initialize bool
}

func NewSwitch(c config.Switch) *Switch {
	var tlsCfg *tls.Config
	var server libol.SocketServer

	if c.Cert.KeyFile != "" && c.Cert.CrtFile != "" {
		cer, err := tls.LoadX509KeyPair(c.Cert.CrtFile, c.Cert.KeyFile)
		if err != nil {
			libol.Error("NewSwitch: %s", err)
		}
		tlsCfg = &tls.Config{Certificates: []tls.Certificate{cer}}
	}
	if c.Protocol == "kcp" {
		server = libol.NewKcpServer(c.Listen, nil)
	} else {
		server = libol.NewTcpServer(c.Listen, tlsCfg)
	}
	v := Switch{
		Conf: c,
		Fire: FireWall{
			Rules: make([]libol.FilterRule, 0, 32),
		},
		worker:     make(map[string]*Worker, 32),
		bridge:     make(map[string]network.Bridger, 32),
		server:     server,
		newTime:    time.Now().Unix(),
		initialize: false,
	}
	return &v
}

func (v *Switch) AddRules(source string, prefix string) {
	libol.Info("Switch.AddRules %s, %s", source, prefix)
	v.Fire.Rules = append(v.Fire.Rules, libol.FilterRule{
		Table:  "filter",
		Chain:  "FORWARD",
		Source: source,
		Dest:   prefix,
		Jump:   "ACCEPT",
	})
	v.Fire.Rules = append(v.Fire.Rules, libol.FilterRule{
		Table:  "nat",
		Chain:  "POSTROUTING",
		Source: source,
		Dest:   prefix,
		Jump:   "MASQUERADE",
	})
	v.Fire.Rules = append(v.Fire.Rules, libol.FilterRule{
		Table:  "nat",
		Chain:  "POSTROUTING",
		Dest:   source,
		Source: prefix,
		Jump:   "MASQUERADE",
	})
}

func (v *Switch) Initialize() {
	v.initialize = true
	if v.Conf.Http != nil {
		v.http = NewHttp(v, v.Conf)
	}
	for _, nCfg := range v.Conf.Network {
		name := nCfg.Name
		brCfg := nCfg.Bridge

		if brCfg.Address != "" {
			source := brCfg.Address
			ifAddr := strings.SplitN(source, "/", 2)[0]
			for i, rt := range nCfg.Routes {
				if rt.NextHop == "" {
					nCfg.Routes[i].NextHop = ifAddr
				}
				rt = nCfg.Routes[i]
				if rt.NextHop != ifAddr {
					continue
				}
				// MASQUERADE
				v.AddRules(source, rt.Prefix)
			}
		}
		v.worker[name] = NewWorker(*nCfg)
		v.bridge[name] = network.NewBridger(brCfg.Provider, brCfg.Name, brCfg.Mtu)
	}

	v.Apps.Auth = app.NewPointAuth(v, v.Conf)
	v.Apps.Request = app.NewWithRequest(v, v.Conf)
	v.Apps.Neighbor = app.NewNeighbors(v, v.Conf)
	v.Apps.OnLines = app.NewOnline(v, v.Conf)

	v.hooks = make([]Hook, 0, 64)
	v.hooks = append(v.hooks, v.Apps.Auth.OnFrame)
	v.hooks = append(v.hooks, v.Apps.Neighbor.OnFrame)
	v.hooks = append(v.hooks, v.Apps.Request.OnFrame)
	v.hooks = append(v.hooks, v.Apps.OnLines.OnFrame)
	for i, h := range v.hooks {
		libol.Debug("Switch.Initialize: k %d, func %p, %s", i, h, libol.FunName(h))
	}

	// Controller
	ctrls.Load(v.Conf.ConfDir + "/ctrl.json")
	if ctrls.Ctrl.Name == "" {
		ctrls.Ctrl.Name = v.Conf.Alias
	}
	ctrls.Ctrl.Switcher = v

	// FireWall
	for _, rule := range v.Conf.FireWall {
		v.Fire.Rules = append(v.Fire.Rules, libol.FilterRule{
			Table:    rule.Table,
			Chain:    rule.Chain,
			Source:   rule.Source,
			Dest:     rule.Dest,
			Jump:     rule.Jump,
			ToSource: rule.ToSource,
			ToDest:   rule.ToDest,
			Comment:  rule.Comment,
			Input:    rule.Input,
			Output:   rule.Output,
		})
	}
	libol.Info("Switch.Initialize total %d rules", len(v.Fire.Rules))
}

func (v *Switch) OnHook(client libol.SocketClient, data []byte) error {
	frame := libol.NewFrameMessage(data)
	for _, h := range v.hooks {
		libol.Log("Worker.onHook: h %p", h)
		if h != nil {
			if err := h(client, frame); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *Switch) OnClient(client libol.SocketClient) error {
	client.SetStatus(libol.CL_CONNECTED)
	libol.Info("Switch.onClient: %s", client.Addr())
	return nil
}

func (v *Switch) ReadClient(client libol.SocketClient, data []byte) error {
	libol.Log("Switch.ReadClient: %s % x", client.Addr(), data)
	if err := v.OnHook(client, data); err != nil {
		libol.Debug("Switch.OnRead: %s dropping by %s", client.Addr(), err)
		return nil
	}

	private := client.Private()
	if private != nil {
		point := private.(*models.Point)
		dev := point.Device
		if point == nil || dev == nil {
			return libol.NewErr("Tap devices is nil")
		}
		if _, err := dev.Write(data); err != nil {
			libol.Error("Worker.OnRead: %s", err)
			return err
		}
		return nil
	}
	return libol.NewErr("%s Point not found.", client)
}

func (v *Switch) OnClose(client libol.SocketClient) error {
	libol.Info("Switch.OnClose: %s", client.Addr())

	uuid := storage.Point.GetUUID(client.Addr())
	if storage.Point.GetAddr(uuid) == client.Addr() { // not has newer
		storage.Network.FreeAddr(uuid)
	}
	storage.Point.Del(client.Addr())

	return nil
}

func (v *Switch) Start() error {
	v.lock.Lock()
	defer v.lock.Unlock()

	libol.Debug("Switch.Start")
	if !v.initialize {
		v.Initialize()
	}

	for _, nCfg := range v.Conf.Network {
		if br, ok := v.bridge[nCfg.Name]; ok {
			brCfg := nCfg.Bridge
			br.Open(brCfg.Address)
		}
	}
	go v.server.Accept()
	call := libol.ServerListener{
		OnClient: v.OnClient,
		OnClose:  v.OnClose,
		ReadAt:   v.ReadClient,
	}
	go v.server.Loop(call)
	for _, w := range v.worker {
		w.Start(v)
	}
	if v.http != nil {
		go v.http.Start()
	}
	go ctrls.Ctrl.Start()

	v.Fire.Start()
	return nil
}

func (v *Switch) Stop() error {
	v.lock.Lock()
	defer v.lock.Unlock()

	libol.Debug("Switch.Stop")
	v.Fire.Stop()
	ctrls.Ctrl.Stop()
	if v.bridge == nil {
		return libol.NewErr("already closed")
	}
	for _, nCfg := range v.Conf.Network {
		if br, ok := v.bridge[nCfg.Name]; ok {
			brCfg := nCfg.Bridge
			_ = br.Close()
			delete(v.bridge, brCfg.Name)
		}
	}
	if v.http != nil {
		v.http.Shutdown()
		v.http = nil
	}
	v.server.Close()
	for _, w := range v.worker {
		w.Stop()
	}
	return nil
}

func (v *Switch) Alias() string {
	return v.Conf.Alias
}

func (v *Switch) UpTime() int64 {
	return time.Now().Unix() - v.newTime
}

func (v *Switch) Server() libol.SocketServer {
	return v.server
}

func (v *Switch) NewTap(tenant string) (network.Taper, error) {
	v.lock.RLock()
	defer v.lock.RUnlock()
	libol.Debug("Worker.NewTap")

	br, ok := v.bridge[tenant]
	if !ok {
		return nil, libol.NewErr("Not found bridge %s", tenant)
	}
	dev, err := network.NewTaper(br.Type(), tenant, network.TapConfig{Type: network.TAP})
	if err != nil {
		libol.Error("Worker.NewTap: %s", err)
		return nil, err
	}
	mtu := br.Mtu()
	dev.SetMtu(mtu)
	dev.Up()
	_ = br.AddSlave(dev)
	libol.Info("Worker.NewTap: %s on %s", dev.Name(), tenant)
	return dev, nil
}

func (v *Switch) FreeTap(dev network.Taper) error {
	br, ok := v.bridge[dev.Tenant()]
	if !ok {
		return libol.NewErr("Not found bridge %s", dev.Tenant())
	}
	_ = br.DelSlave(dev)
	libol.Info("Worker.FreeTap: %s", dev.Name())
	return nil
}

func (v *Switch) UUID() string {
	if v.uuid == "" {
		v.uuid = libol.GenToken(32)
	}
	return v.uuid
}

func (v *Switch) AddLink(tenant string, c *config.Point) {

}

func (v *Switch) DelLink(tenant, addr string) {

}

func (v *Switch) ReadTap(dev network.Taper, readAt func(p []byte) error) {
	defer dev.Close()
	libol.Info("Switch.ReadTap: %s", dev.Name())

	data := make([]byte, libol.MAXBUF)
	for {
		n, err := dev.Read(data)
		if err != nil {
			libol.Error("Switch.ReadTap: %s", err)
			break
		}
		libol.Log("Switch.ReadTap: % x\n", data)
		if err := readAt(data[:n]); err != nil {
			libol.Error("Switch.ReadTap: do-recv %s %s", dev.Name(), err)
			break
		}
	}
}

func (v *Switch) OffClient(client libol.SocketClient) {
	libol.Info("Switch.OffClient: %s", client)
	if v.server != nil {
		v.server.OffClient(client)
	}
}

func (v *Switch) Config() *config.Switch {
	return &v.Conf
}
