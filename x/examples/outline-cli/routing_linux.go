// Copyright 2023 Jigsaw Operations LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

var ipRule *netlink.Rule = nil

func startRouting(config *RoutingConfig) error {
	if err := setupRoutingTable(config.RoutingTableID, config.TunDeviceName, config.TunGatewayCIDR, config.TunDeviceIP); err != nil {
		return err
	}
}

func stopRouting(routingTable int) {
	if err := cleanUpRoutingTable(routingTable); err != nil {
		logging.Err.Printf("failed to clean up routing table '%v': %v\n", routingTable, err)
	}
}

func setupRoutingTable(routingTable int, tunName, gwSubnet string, tunIP string) error {
	tun, err := netlink.LinkByName(tunName)
	if err != nil {
		return fmt.Errorf("failed to find tun device '%s': %w", tunName, err)
	}

	dst, err := netlink.ParseIPNet(gwSubnet)
	if err != nil {
		return fmt.Errorf("failed to parse gateway '%s': %w", gwSubnet, err)
	}

	r := netlink.Route{
		LinkIndex: tun.Attrs().Index,
		Table:     routingTable,
		Dst:       dst,
		Src:       net.ParseIP(tunIP),
		Scope:     netlink.SCOPE_LINK,
	}

	if err = netlink.RouteAdd(&r); err != nil {
		return fmt.Errorf("failed to add routing entry '%v' -> '%v': %w", r.Src, r.Dst, err)
	}
	logging.Info.Printf("routing traffic from %v to %v through nic %v\n", r.Src, r.Dst, r.LinkIndex)

	r = netlink.Route{
		LinkIndex: tun.Attrs().Index,
		Table:     routingTable,
		Gw:        dst.IP,
	}

	if err := netlink.RouteAdd(&r); err != nil {
		return fmt.Errorf("failed to add gateway routing entry '%v': %w", r.Gw, err)
	}
	logging.Info.Printf("routing traffic via gw %v through nic %v...\n", r.Gw, r.LinkIndex)

	return nil
}

func cleanUpRoutingTable(routingTable int) error {
	filter := netlink.Route{Table: routingTable}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &filter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return fmt.Errorf("failed to list entries in routing table '%v': %w", routingTable, err)
	}

	var rtDelErr error = nil
	for _, route := range routes {
		if err := netlink.RouteDel(&route); err != nil {
			rtDelErr = errors.Join(rtDelErr, fmt.Errorf("failed to remove routing entry: %w", err))
		}
	}
	if rtDelErr == nil {
		logging.Info.Printf("routing table '%v' has been cleaned up\n", routingTable)
	}
	return rtDelErr
}
