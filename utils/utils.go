package utils

import "net/netip"

func MakePrefixFromAddr(addr netip.Addr) netip.Prefix {
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32)
	} else {
		return netip.PrefixFrom(addr, 128)
	}
}
