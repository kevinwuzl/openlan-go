package config

import (
	"flag"
	"fmt"
	"github.com/lightstar-dev/openlan-go/libol"
)

type VSwitch struct {
	TcpListen  string      `json:"Listen"`
	Verbose    int         `json:"Verbose"`
	HttpListen string      `json:"Http"`
	IfMtu      int         `json:"IfMtu"`
	IfAddr     string      `json:"IfAddr"`
	BrName     string      `json:"IfBridge"`
	Token      string      `json:"AdminToken"`
	TokenFile  string      `json:"AdminFile"`
	Password   string      `json:"AuthFile"`
	Redis      RedisConfig `json:"Redis"`
	LogFile    string      `json:"LogFile"`
	CrtFile    string      `json:"CrtFile"`
	KeyFile    string      `json:"KeyFile"`
	Links      []*Point    `json:"Links"`
	SaveFile   string      `json:"-"`
}

type RedisConfig struct {
	Enable bool   `json:"Enable"`
	Addr   string `json:"Addr"`
	Auth   string `json:"Auth"`
	Db     int    `json:"Database"`
}

var VSwitchDefault = VSwitch{
	BrName:     "",
	Verbose:    libol.INFO,
	HttpListen: "",
	TcpListen:  "0.0.0.0:10002",
	Token:      "",
	TokenFile:  ".vswitch.token",
	Password:   ".password",
	IfMtu:      1518,
	IfAddr:     "",
	Redis: RedisConfig{
		Addr:   "127.0.0.1",
		Auth:   "",
		Db:     0,
		Enable: false,
	},
	LogFile:  ".vswitch.error",
	SaveFile: ".vswitch.json",
	CrtFile:  "",
	KeyFile:  "",
	Links:    nil,
}

func NewVSwitch() (c *VSwitch) {
	c = &VSwitch{
		Redis:   VSwitchDefault.Redis,
		LogFile: VSwitchDefault.LogFile,
	}

	flag.IntVar(&c.Verbose, "verbose", VSwitchDefault.Verbose, "open verbose")
	flag.StringVar(&c.HttpListen, "http:addr", VSwitchDefault.HttpListen, "the http listen on")
	flag.StringVar(&c.TcpListen, "vs:addr", VSwitchDefault.TcpListen, "the server listen on")
	flag.StringVar(&c.Token, "admin:token", VSwitchDefault.Token, "Administrator token")
	flag.StringVar(&c.TokenFile, "admin:file", VSwitchDefault.TokenFile, "The file administrator token saved to")
	flag.StringVar(&c.Password, "auth:file", VSwitchDefault.Password, "The file password loading from.")
	flag.IntVar(&c.IfMtu, "if:mtu", VSwitchDefault.IfMtu, "the interface MTU include ethernet")
	flag.StringVar(&c.IfAddr, "if:addr", VSwitchDefault.IfAddr, "the interface address")
	flag.StringVar(&c.BrName, "if:br", VSwitchDefault.BrName, "the bridge name")
	flag.StringVar(&c.SaveFile, "conf", VSwitchDefault.SaveFile, "The configuration file")
	flag.StringVar(&c.CrtFile, "tls:crt", VSwitchDefault.CrtFile, "The X509 certificate file for TLS")
	flag.StringVar(&c.KeyFile, "tls:key", VSwitchDefault.KeyFile, "The X509 certificate key for TLS")

	flag.Parse()
	c.Default()
	if err := c.Load(); err != nil {
		libol.Error("NewVSwitch.load %s", err)
	}

	libol.Init(c.LogFile, c.Verbose)
	c.Save(fmt.Sprintf("%s.cur", c.SaveFile))

	str, err := libol.Marshal(c, false)
	if err != nil {
		libol.Error("NewVSwitch.json error: %s", err)
	}
	libol.Debug("NewVSwitch.json: %s", str)

	return
}

func (c *VSwitch) Right() {
	RightAddr(&c.TcpListen, 10002)
	RightAddr(&c.HttpListen, 10000)
}

func (c *VSwitch) Default() {
	c.Right()
	// TODO reset zero value to default
}

func (c *VSwitch) Save(file string) error {
	if file == "" {
		file = c.SaveFile
	}

	return libol.MarshalSave(c, file, true)
}

func (c *VSwitch) Load() error {
	if err := libol.UnmarshalLoad(c, c.SaveFile); err != nil {
		return err
	}

	if c.Links != nil {
		for _, link := range c.Links {
			link.Default()
		}
	}
	return nil
}


func init() {
	VSwitchDefault.Right()
}
