package founat

import (
	"crypto/sha1"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"sync"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

// Prefixes for Foo-over-UDP tunnel link names
const (
	FoU4LinkPrefix = "fou4_"
	FoU6LinkPrefix = "fou6_"
)

const fouDummy = "fou-dummy"

func fouName(addr net.IP) string {
	if v4 := addr.To4(); v4 != nil {
		return fmt.Sprintf("%s%x", FoU4LinkPrefix, []byte(v4))
	}

	hash := sha1.Sum([]byte(addr))
	return fmt.Sprintf("%s%x", FoU6LinkPrefix, hash[:4])
}

func modProbe(module string) error {
	out, err := exec.Command("/sbin/modprobe", module).CombinedOutput()
	if err != nil {
		return fmt.Errorf("modprobe %s failed with %w: %s", module, err, string(out))
	}
	return nil
}

// FoUTunnel represents the interface for Foo-over-UDP tunnels.
// Methods are idempotent; i.e. they can be called multiple times.
type FoUTunnel interface {
	// Init starts FoU listening socket.
	Init() error

	// AddPeer setups tunnel devices to the given peer and returns them.
	// If FoUTunnel does not setup for the IP family of the given address,
	// this returns ErrIPFamilyMismatch error.
	AddPeer(net.IP) (netlink.Link, error)

	// DelPeer deletes tunnel for the peer, if any.
	DelPeer(net.IP) error
}

// NewFoUTunnel creates a new FoUTunnel.
// sport/dport is the UDP port to receive FoU packets.
// localIPv4 is the local IPv4 address of the IPIP tunnel.  This can be nil.
// localIPv6 is the same as localIPv4 for IPv6.
func NewFoUTunnel(sport, dport int, localIPv4, localIPv6 net.IP) FoUTunnel {
	if localIPv4 != nil && localIPv4.To4() == nil {
		panic("invalid IPv4 address")
	}
	if localIPv6 != nil && localIPv6.To4() != nil {
		panic("invalid IPv6 address")
	}
	return &fouTunnel{
		sport:  sport,
		dport:  dport,
		local4: localIPv4,
		local6: localIPv6,
	}
}

type fouTunnel struct {
	sport  int
	dport  int
	local4 net.IP
	local6 net.IP

	mu sync.Mutex
}

func (t *fouTunnel) Init() error {
	// avoid double initialization in case the program restarts
	_, err := netlink.LinkByName(fouDummy)
	if err == nil {
		return nil
	}
	if _, ok := err.(netlink.LinkNotFoundError); !ok {
		return err
	}

	if t.local4 != nil {
		if err := modProbe("fou"); err != nil {
			return fmt.Errorf("failed to load fou module: %w", err)
		}
		err := netlink.FouAdd(netlink.Fou{
			Family:    netlink.FAMILY_V4,
			Protocol:  4, // IPv4 over IPv4 (so-called IPIP)
			Port:      t.dport,
			EncapType: netlink.FOU_ENCAP_DIRECT,
		})
		if err != nil {
			return fmt.Errorf("netlink: fou add failed: %w", err)
		}
		if _, err := sysctl.Sysctl("net.ipv4.conf.default.rp_filter", "0"); err != nil {
			return fmt.Errorf("setting net.ipv4.conf.default.rp_filter=0 failed: %w", err)
		}
		if _, err := sysctl.Sysctl("net.ipv4.conf.all.rp_filter", "0"); err != nil {
			return fmt.Errorf("setting net.ipv4.conf.all.rp_filter=0 failed: %w", err)
		}
		if err := ip.EnableIP4Forward(); err != nil {
			return fmt.Errorf("failed to enable IPv4 forwarding: %w", err)
		}

		ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
		if err != nil {
			return err
		}
		// workaround for kube-proxy's double NAT problem
		rulespec := []string{
			"-p", "udp", "--dport", strconv.Itoa(t.dport), "-j", "CHECKSUM", "--checksum-fill",
		}
		if err := ipt.Insert("mangle", "POSTROUTING", 1, rulespec...); err != nil {
			return fmt.Errorf("failed to setup mangle table: %w", err)
		}
	}
	if t.local6 != nil {
		if err := modProbe("fou6"); err != nil {
			return fmt.Errorf("failed to load fou6 module: %w", err)
		}
		err := netlink.FouAdd(netlink.Fou{
			Family:    netlink.FAMILY_V6,
			Protocol:  41, // IPv6 over IPv6 (so-called SIT)
			Port:      t.dport,
			EncapType: netlink.FOU_ENCAP_DIRECT,
		})
		if err != nil {
			return fmt.Errorf("netlink: fou add failed: %w", err)
		}
		if err := ip.EnableIP6Forward(); err != nil {
			return fmt.Errorf("failed to enable IPv6 forwarding: %w", err)
		}

		ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
		if err != nil {
			return err
		}
		// workaround for kube-proxy's double NAT problem
		rulespec := []string{
			"-p", "udp", "--dport", strconv.Itoa(t.dport), "-j", "CHECKSUM", "--checksum-fill",
		}
		if err := ipt.Insert("mangle", "POSTROUTING", 1, rulespec...); err != nil {
			return fmt.Errorf("failed to setup mangle table: %w", err)
		}
	}

	attrs := netlink.NewLinkAttrs()
	attrs.Name = fouDummy
	return netlink.LinkAdd(&netlink.Dummy{LinkAttrs: attrs})
}

func (t *fouTunnel) AddPeer(addr net.IP) (netlink.Link, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if v4 := addr.To4(); v4 != nil {
		return t.addPeer4(v4)
	}
	return t.addPeer6(addr)
}

func (t *fouTunnel) addPeer4(addr net.IP) (netlink.Link, error) {
	if t.local4 == nil {
		return nil, ErrIPFamilyMismatch
	}

	linkName := fouName(addr)

	link, err := netlink.LinkByName(linkName)
	if err == nil {
		return link, nil
	}
	if _, ok := err.(netlink.LinkNotFoundError); !ok {
		return nil, fmt.Errorf("netlink: failed to get link: %w", err)
	}

	attrs := netlink.NewLinkAttrs()
	attrs.Name = linkName
	attrs.Flags = net.FlagUp
	link = &netlink.Iptun{
		LinkAttrs:  attrs,
		Ttl:        225,
		EncapType:  netlink.FOU_ENCAP_DIRECT,
		EncapDport: uint16(t.dport),
		EncapSport: uint16(t.sport),
		Remote:     addr,
		Local:      t.local4,
	}
	if err := netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("netlink: failed to add fou link: %w", err)
	}

	if err := setupIPIPDevices(true, false); err != nil {
		return nil, fmt.Errorf("netlink: failed to setup ipip device: %w", err)
	}

	return netlink.LinkByName(linkName)
}

// setupIPIPDevices ensures the specified v4 and/or v6 devices are created
//
// Calling this function may result in tunl0 (v4) or ip6tnl0 (v6) fallback
// interfaces being created as a result of loading the ipip and ip6_tunnel
// kernel modules by fou tunnel interfaces. These are catch-all
// interfaces for the ipip decapsulation stack. By default, these interfaces
// will be created in new network namespaces, but this behavior can be disabled
// by setting net.core.fb_tunnels_only_for_init_net = 2.
//
// If present, tunl0 is renamed to egress_tunl and ip6tnl0 is
// renamed to egress_ip6tnl. This is to communicate to the user that this plugin has
// taken control of the encapsulation stack on the netns, as it currently doesn't
// explicitly support sharing it with other tools/CNIs. Fallback devices are left
// unused for production traffic. Only devices that were explicitly created are used.
func setupIPIPDevices(ipv4, ipv6 bool) error {
	ipip4Device := "egress_ipip4"
	ipip6Device := "egress_ipip6"
	if ipv4 {
		// Set up IPv4 tunnel device if requested.
		if err := setupDevice(&netlink.Iptun{
			LinkAttrs: netlink.LinkAttrs{Name: ipip4Device},
			FlowBased: true,
		}); err != nil {
			return fmt.Errorf("creating %s: %w", ipip4Device, err)
		}

		// Rename fallback device created by potential kernel module load after
		// creating tunnel interface.
		if err := renameDevice("tunl0", "egress_tunl"); err != nil {
			return fmt.Errorf("renaming fallback device %s: %w", "tunl0", err)
		}
	} else {
		if err := removeDevice(ipip4Device); err != nil {
			return fmt.Errorf("removing %s: %w", ipip4Device, err)
		}
	}

	if ipv6 {
		// Set up IPv6 tunnel device if requested.
		if err := setupDevice(&netlink.Ip6tnl{
			LinkAttrs: netlink.LinkAttrs{Name: ipip6Device},
			FlowBased: true,
		}); err != nil {
			return fmt.Errorf("creating %s: %w", ipip6Device, err)
		}

		// Rename fallback device created by potential kernel module load after
		// creating tunnel interface.
		if err := renameDevice("ip6tnl0", "egress_ip6tnl"); err != nil {
			return fmt.Errorf("renaming fallback device %s: %w", "tunl0", err)
		}
	} else {
		if err := removeDevice(ipip6Device); err != nil {
			return fmt.Errorf("removing %s: %w", ipip6Device, err)
		}
	}

	return nil
}

// setupDevice creates and configures a device based on the given netlink attrs.
func setupDevice(attrs netlink.Link) error {
	name := attrs.Attrs().Name

	// Reuse existing tunnel interface created by previous runs.
	l, err := netlink.LinkByName(name)
	if err != nil {
		if err := netlink.LinkAdd(attrs); err != nil {
			return fmt.Errorf("netlink: failed to create device %s: %w", name, err)
		}

		// Fetch the link we've just created.
		l, err = netlink.LinkByName(name)
		if err != nil {
			return fmt.Errorf("netlink: failed to retrieve created device %s: %w", name, err)
		}
	}

	if err := configureDevice(l); err != nil {
		return fmt.Errorf("failed to set up device %s: %w", l.Attrs().Name, err)
	}

	return nil
}

// configureDevice puts the given link into the up state
func configureDevice(link netlink.Link) error {
	ifName := link.Attrs().Name

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("netlink: failed to set link %s up: %w", ifName, err)
	}
	return nil
}

// removeDevice removes the device with the given name. Returns error if the
// device exists but was unable to be removed.
func removeDevice(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil
	}

	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("netlink: failed to remove device %s: %w", name, err)
	}

	return nil
}

// renameDevice renames a network device from and to a given value. Returns nil
// if the device does not exist.
func renameDevice(from, to string) error {
	link, err := netlink.LinkByName(from)
	if err != nil {
		return nil
	}

	if err := netlink.LinkSetName(link, to); err != nil {
		return fmt.Errorf("netlink: failed to rename device %s to %s: %w", from, to, err)
	}

	return nil
}

func (t *fouTunnel) addPeer6(addr net.IP) (netlink.Link, error) {
	if t.local6 == nil {
		return nil, ErrIPFamilyMismatch
	}

	linkName := fouName(addr)

	link, err := netlink.LinkByName(linkName)
	if err == nil {
		return link, nil
	}
	if _, ok := err.(netlink.LinkNotFoundError); !ok {
		return nil, fmt.Errorf("netlink: failed to get link: %w", err)
	}

	attrs := netlink.NewLinkAttrs()
	attrs.Name = linkName
	attrs.Flags = net.FlagUp
	link = &netlink.Ip6tnl{
		LinkAttrs:  attrs,
		Ttl:        225,
		EncapType:  netlink.FOU_ENCAP_DIRECT,
		EncapDport: uint16(t.dport),
		EncapSport: uint16(t.sport),
		Remote:     addr,
		Local:      t.local6,
	}
	if err := netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("netlink: failed to add fou6 link: %w", err)
	}

	if err := setupIPIPDevices(false, true); err != nil {
		return nil, fmt.Errorf("netlink: failed to setup ipip device: %w", err)
	}

	return netlink.LinkByName(linkName)
}

func (t *fouTunnel) DelPeer(addr net.IP) error {
	linkName := fouName(addr)

	link, err := netlink.LinkByName(linkName)
	if err == nil {
		return netlink.LinkDel(link)
	}

	if _, ok := err.(netlink.LinkNotFoundError); ok {
		return nil
	}
	return err
}
