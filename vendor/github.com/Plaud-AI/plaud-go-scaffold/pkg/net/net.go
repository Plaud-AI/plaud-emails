package net

import (
	"net"
	"os"
)

// GetHostName 获取主机名, 如果主机名不存在, 则返回第一个非环回IPv4地址
func GetHostName() string {
	host, _ := os.Hostname()
	if host != "" {
		return host
	}
	return FirstNonLoopbackIPv4()
}

// FirstNonLoopbackIPv4 获取第一个非环回IPv4地址
func FirstNonLoopbackIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if (iface.Flags & net.FlagUp) == 0 {
			continue
		}
		if (iface.Flags & net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return ""
}
