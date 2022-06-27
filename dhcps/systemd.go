package dhcps

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dropbox/godropbox/errors"
	"github.com/pritunl/mongo-go-driver/bson/primitive"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"github.com/pritunl/pritunl-cloud/node"
	"github.com/pritunl/pritunl-cloud/paths"
	"github.com/pritunl/pritunl-cloud/settings"
	"github.com/pritunl/pritunl-cloud/systemd"
	"github.com/pritunl/pritunl-cloud/utils"
	"github.com/pritunl/pritunl-cloud/vm"
	"github.com/pritunl/pritunl-cloud/vpc"
	"github.com/pritunl/pritunl-cloud/zone"
)

const systemdTemplate = `[Unit]
Description=Pritunl Cloud %s
After=network.target

[Service]
Environment=CONFIG=%s
Type=simple
User=root
ExecStart=/usr/sbin/ip netns exec %s %s %s
PrivateTmp=true
ProtectHome=true
ProtectSystem=full
ProtectHostname=true
ProtectKernelTunables=true
NetworkNamespacePath=/var/run/netns/%s
AmbientCapabilities=CAP_NET_BIND_SERVICE
`

func UpdateEbtables(vmId primitive.ObjectID, namespace string) (err error) {
	iface := vm.GetIface(vmId, 0)

	_, err = utils.ExecCombinedOutputLogged(
		nil,
		"ip", "netns", "exec", namespace,
		"ebtables",
		"-I", "OUTPUT",
		"-o", iface,
		"-p", "IPv4",
		"--ip-protocol", "udp",
		"--ip-sport", "67",
		"-j", "ACCEPT",
	)
	if err != nil {
		return
	}
	_, err = utils.ExecCombinedOutputLogged(
		nil,
		"ip", "netns", "exec", namespace,
		"ebtables",
		"-A", "OUTPUT",
		"-p", "IPv4",
		"--ip-protocol", "udp",
		"--ip-sport", "67",
		"-j", "DROP",
	)
	if err != nil {
		return
	}

	_, err = utils.ExecCombinedOutputLogged(
		nil,
		"ip", "netns", "exec", namespace,
		"ebtables",
		"-I", "OUTPUT",
		"-o", iface,
		"-p", "IPv6",
		"--ip6-protocol", "udp",
		"--ip6-sport", "547",
		"-j", "ACCEPT",
	)
	if err != nil {
		return
	}
	_, err = utils.ExecCombinedOutputLogged(
		nil,
		"ip", "netns", "exec", namespace,
		"ebtables",
		"-A", "OUTPUT",
		"-p", "IPv6",
		"--ip6-protocol", "udp",
		"--ip6-sport", "547",
		"-j", "DROP",
	)
	if err != nil {
		return
	}

	_, err = utils.ExecCombinedOutputLogged(
		nil,
		"ip", "netns", "exec", namespace,
		"ebtables",
		"-I", "OUTPUT",
		"-o", iface,
		"-p", "IPv6",
		"--ip6-protocol", "ipv6-icmp",
		"--ip6-icmp-type", "134",
		"-j", "ACCEPT",
	)
	if err != nil {
		return
	}
	_, err = utils.ExecCombinedOutputLogged(
		nil,
		"ip", "netns", "exec", namespace,
		"ebtables",
		"-A", "OUTPUT",
		"-p", "IPv6",
		"--ip6-protocol", "ipv6-icmp",
		"--ip6-icmp-type", "134",
		"-j", "DROP",
	)
	if err != nil {
		return
	}

	return
}

func WriteService(vmId primitive.ObjectID, namespace string,
	config interface{}) (err error) {

	param := ""
	unitPath := ""

	curPath, err := os.Executable()
	if err != nil {
		err = &errortypes.ParseError{
			errors.Wrap(err, "dhcps: Failed to get executable path"),
		}
		return
	}

	confData, err := json.Marshal(config)
	if err != nil {
		err = &errortypes.ParseError{
			errors.Wrap(err, "dhcps: Failed to marshal config"),
		}
		return
	}

	switch config.(type) {
	case *Server4:
		param = "dhcp4-server"
		unitPath = paths.GetUnitPathDhcp4(vmId)
		break
	case *Server6:
		param = "dhcp6-server"
		unitPath = paths.GetUnitPathDhcp6(vmId)
		break
	case *ServerNdp:
		param = "ndp-server"
		unitPath = paths.GetUnitPathNdp(vmId)
		break
	default:
		err = &errortypes.TypeError{
			errors.New("dhcps: Unknown config type"),
		}
		return
	}

	output := fmt.Sprintf(
		systemdTemplate,
		param,
		string(confData),
		namespace,
		curPath,
		param,
		namespace,
	)

	err = utils.CreateWrite(unitPath, output, 0644)
	if err != nil {
		return
	}

	return
}

func Start(db *database.Database, virt *vm.VirtualMachine) (err error) {
	namespace := vm.GetNamespace(virt.Id, 0)

	zne, err := zone.Get(db, node.Self.Zone)
	if err != nil {
		return
	}

	if virt.NetworkAdapters == nil || len(virt.NetworkAdapters) < 1 {
		err = &errortypes.ParseError{
			errors.New("dhcps: Missing virt network adapter"),
		}
		return
	}
	vpcId := virt.NetworkAdapters[0].Vpc
	subnetId := virt.NetworkAdapters[0].Subnet

	vc, err := vpc.Get(db, vpcId)
	if err != nil {
		return
	}

	vcNet, err := vc.GetNetwork()
	if err != nil {
		return
	}

	cidr, _ := vcNet.Mask.Size()
	addr, gatewayAddr, err := vc.GetIp(db, subnetId, virt.Id)
	if err != nil {
		return
	}

	addr6 := vc.GetIp6(addr)
	gatewayAddr6 := vc.GetIp6(gatewayAddr)

	jumboFramesExternal := node.Self.JumboFrames
	jumboFramesInternal := node.Self.JumboFrames ||
		node.Self.JumboFramesInternal
	vxlan := zne.NetworkMode == zone.VxlanVlan
	mtu := 0

	if jumboFramesExternal || jumboFramesInternal || vxlan {
		mtuSizeInternal := 0

		if jumboFramesInternal {
			mtuSizeInternal = settings.Hypervisor.JumboMtu
		} else {
			mtuSizeInternal = settings.Hypervisor.NormalMtu
		}

		if vxlan {
			mtuSizeInternal -= 54
		}

		mtu = mtuSizeInternal
	}

	server4 := &Server4{
		Iface:     "br0",
		ClientIp:  addr.String(),
		GatewayIp: gatewayAddr.String(),
		PrefixLen: cidr,
		DnsServers: []string{
			settings.Hypervisor.DnsServerPrimary,
			settings.Hypervisor.DnsServerSecondary,
		},
		Mtu:      mtu,
		Lifetime: 60,
	}
	server6 := &Server6{
		Iface:     "br0",
		ClientIp:  addr6.String(),
		GatewayIp: gatewayAddr6.String(),
		PrefixLen: 64,
		DnsServers: []string{
			settings.Hypervisor.DnsServerPrimary6,
			settings.Hypervisor.DnsServerSecondary6,
		},
		Mtu:      mtu,
		Lifetime: 60,
	}
	serverNdp := &ServerNdp{
		Iface:     "br0",
		ClientIp:  addr6.String(),
		GatewayIp: gatewayAddr6.String(),
		PrefixLen: 64,
		DnsServers: []string{
			settings.Hypervisor.DnsServerPrimary6,
			settings.Hypervisor.DnsServerSecondary6,
		},
		Mtu:      mtu,
		Lifetime: 60,
		Delay:    3,
	}

	err = UpdateEbtables(virt.Id, namespace)
	if err != nil {
		return
	}

	unitServer4 := paths.GetUnitNameDhcp4(virt.Id)
	unitServer6 := paths.GetUnitNameDhcp6(virt.Id)
	unitServerNdp := paths.GetUnitNameNdp(virt.Id)

	_ = systemd.Stop(unitServer4)
	_ = systemd.Stop(unitServer6)
	_ = systemd.Stop(unitServerNdp)

	err = WriteService(virt.Id, namespace, server4)
	if err != nil {
		return
	}
	err = WriteService(virt.Id, namespace, server6)
	if err != nil {
		return
	}
	err = WriteService(virt.Id, namespace, serverNdp)
	if err != nil {
		return
	}

	err = systemd.Reload()
	if err != nil {
		return
	}

	err = systemd.Start(unitServer4)
	if err != nil {
		return
	}
	err = systemd.Start(unitServer6)
	if err != nil {
		return
	}
	err = systemd.Start(unitServerNdp)
	if err != nil {
		return
	}

	return
}