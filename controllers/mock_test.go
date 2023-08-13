package controllers

import (
	"net"
	"sync"

	"github.com/vishvananda/netlink"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/founat"
)

type mockFoUTunnel struct {
	mu    sync.Mutex
	peers map[string]bool
}

var _ founat.FoUTunnel = &mockFoUTunnel{}

func (t *mockFoUTunnel) Init() error {
	panic("not implemented")
}

func (t *mockFoUTunnel) AddPeer(ip net.IP) (netlink.Link, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.peers[ip.String()] = true
	return nil, nil
}

func (t *mockFoUTunnel) DelPeer(ip net.IP) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.peers, ip.String())
	return nil
}

func (t *mockFoUTunnel) GetPeers() map[string]bool {
	m := make(map[string]bool)

	t.mu.Lock()
	defer t.mu.Unlock()

	for k := range t.peers {
		m[k] = true
	}
	return m
}

type mockEgress struct {
	mu  sync.Mutex
	ips map[string]bool
}

var _ founat.Egress = &mockEgress{}

func (e *mockEgress) Init() error {
	panic("not implemented")
}

func (e *mockEgress) AddClient(ip net.IP, _ netlink.Link) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.ips[ip.String()] = true
	return nil
}

func (e *mockEgress) GetClients() map[string]bool {
	m := make(map[string]bool)

	e.mu.Lock()
	defer e.mu.Unlock()

	for k := range e.ips {
		m[k] = true
	}
	return m
}
