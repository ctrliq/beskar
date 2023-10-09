// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package netutil

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// RouteGetSourceAddress retrieves the source IP address used
// to reach the destination.
func RouteGetSourceAddress(dest string) (string, error) {
	ip := net.ParseIP(dest)
	if ip == nil {
		ips, err := net.LookupIP(dest)
		if err != nil {
			return "", fmt.Errorf("host lookup failed for %s: %w", dest, err)
		}
		ip = ips[0]
	}

	h, err := netlink.NewHandle()
	if err != nil {
		return "", fmt.Errorf("while getting netlink handle: %w", err)
	}
	defer h.Close()

	r, err := h.RouteGet(ip)
	if err != nil {
		return "", fmt.Errorf("while getting route to destination %s: %w", dest, err)
	}
	for i := range r {
		return r[i].Src.String(), nil
	}
	return "", fmt.Errorf("could not determine source IP address to reach %s", dest)
}

func LocalIPs() ([]net.IP, error) {
	var ips []net.IP

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("network interface lookup error: %w", err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && (ipNet.IP.IsGlobalUnicast() || ipNet.IP.IsLoopback()) {
			ips = append(ips, ipNet.IP)
		}
	}

	return ips, nil
}

func LocalOutboundIPs() ([]net.IP, error) {
	var ips []net.IP

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("network interface lookup error: %w", err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && (ipNet.IP.IsGlobalUnicast()) {
			ips = append(ips, ipNet.IP)
		}

	}

	return ips, nil
}
