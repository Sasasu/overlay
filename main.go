package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

var (
	tTincup, _ = template.New("tincup").Parse(`#!/bin/sh
ip link set $INTERFACE up
ip addr add {{.TSubnet}} dev $INTERFACE
`)

	tHost, _ = template.New("host").Parse(`
{{- if .Address -}}
Address = {{.Address}}
{{end -}}
Subnet = {{.Subnet}}
{{if .RsaPublicKey -}}
-----BEGIN RSA PUBLIC KEY-----
{{.RsaPublicKey -}}
-----END RSA PUBLIC KEY-----
{{end -}}
{{if .Ed25519PublicKey -}}
Ed25519PublicKey = {{.Ed25519PublicKey}}
{{- end -}}
{{if .Port -}}
Port = {{.Port}}
{{- end -}}
`)

	tRsaKey, _ = template.New("rsa_key").Parse(`-----BEGIN RSA PRIVATE KEY-----
{{.RsaPrivateKey -}}
-----END RSA PRIVATE KEY-----
`)

	tEd25519Key, _ = template.New("ed25519_key").Parse(`-----BEGIN ED25519 PRIVATE KEY-----
{{.Ed25519PrivateKey -}}
-----END ED25519 PRIVATE KEY-----
`)

	tConfig, _ = template.New("config").Parse(`Name = {{.Name}}
{{- if .CConnectTo -}}
{{- range $i, $v := .CConnectTo}}
ConnectTo = {{$v}}
{{end -}}
{{end -}}
UpnP = yes
`)
)

// Peer is a tinc peer
type Peer struct {
	Name              string `yaml:"name"`
	Address           string `yaml:"address"`
	Port              int    `yaml:"port"`
	Subnet            string `yaml:"subnet"`
	RsaPublicKey      string `yaml:"rsaPublicKey"`
	RsaPrivateKey     string `yaml:"rsaPrivateKey"`
	Ed25519PublicKey  string `yaml:"ed25519PublicKey"`
	Ed25519PrivateKey string `yaml:"ed25519PrivateKey"`

	CSubnet    string
	CName      string
	CDevice    string
	CConnectTo []string

	TSubnet string
}

// Config is just the config
type Config struct {
	Device string `yaml:"device"`
	Name   string `yaml:"name"`
	Subnet string `yaml:"subnet"`
	Peers  []Peer `yaml:"peers"`
}

func (c *Config) init() {
	var connectTo []string
	for i := range c.Peers {
		t := &c.Peers[i]
		t.CSubnet = c.Subnet
		t.CName = c.Name
		t.CDevice = c.Device
		if len(t.Address) != 0 {
			connectTo = append(connectTo, t.Name)
		}

		t.TSubnet = strings.Split(t.Subnet, "/")[0] + "/" + strings.Split(c.Subnet, "/")[1]
	}
	for i := range c.Peers {
		c.Peers[i].CConnectTo = connectTo
	}
}

var (
	file   string
	name   string
	out    string
	config Config
)

func init() {
	flag.StringVar(&file, "file", "./tinc.yaml", "input file")
	flag.StringVar(&file, "f", "./tinc.yaml", "input file (shorthand)")
	flag.StringVar(&name, "name", "", "the peer's name")
	flag.StringVar(&name, "n", "", "the peer's name (shorthand)")
	flag.StringVar(&out, "out", ".", "out put path")
	flag.StringVar(&out, "o", ".", "out put path (shorthand)")
	flag.Parse()
}

func main() {
	if len(name) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	f, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(2)
	}

	err = yaml.Unmarshal(f, &config)
	if err != nil {
		fmt.Println(err)
		os.Exit(3)
	}
	config.init()

	var peer *Peer
	for i := range config.Peers {
		if config.Peers[i].Name == name {
			peer = &config.Peers[i]
			break
		}
	}
	if peer == nil {
		panic(fmt.Sprintf("name %s not find in peers", name))
	}

	writeConfig(peer)
}

func writeConfig(peer *Peer) {
	_ = os.MkdirAll(filepath.Join(out, config.Name), 0755)
	_ = os.MkdirAll(filepath.Join(out, config.Name, "hosts"), 0755)

	rander("tinc-up", 0755, tTincup, peer)
	for _, i := range config.Peers {
		rander(filepath.Join("hosts", i.Name), 0644, tHost, i)
	}

	rander("rsa_key.priv", 0600, tRsaKey, peer)
	rander("ed25519_key.priv", 0600, tEd25519Key, peer)
	rander("tinc.conf", 0644, tConfig, peer)
}

func rander(name string, mode os.FileMode, t *template.Template, d interface{}) {
	path := filepath.Join(out, config.Name)
	file, err := os.OpenFile(filepath.Join(path, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	defer file.Close()
	if err != nil {
		panic(err)
	}
	if err := t.Execute(file, d); err != nil {
		panic(err)
	}
}
