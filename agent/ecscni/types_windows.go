//go:build windows
// +build windows

// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package ecscni

import (
	"time"

	"github.com/containernetworking/cni/pkg/types"
)

const (
	// ECSVPCENIPluginName is the name of the vpc-eni plugin.
	ECSVPCENIPluginName = "vpc-eni"
	// ECSVPCENIPluginExecutable is the name of vpc-eni executable.
	ECSVPCENIPluginExecutable = "vpc-eni.exe"

	// ECSVPCBridgePluginName is the name of the vpc-bridge plugin.
	ECSVPCBridgePluginName = "vpc-bridge"
	// ECSVPCBridgePluginExecutable is the name of vpc-bridge executable.
	ECSVPCBridgePluginExecutable = "vpc-bridge.exe"

	// TaskHNSNetworkNamePrefix is the prefix of the HNS network used for task ENI.
	TaskHNSNetworkNamePrefix = "task"
	// VPCBridgeHNSNetworkNamePrefix is the prefix of the HNS network used for VPC Bridge mode.
	VPCBridgeHNSNetworkNamePrefix = "vpc-bridge"
	// ECSBridgeNetworkName is the name of the HNS network used as ecs-bridge.
	ECSBridgeNetworkName = "ecs-bridge"

	// windowsDefaultRoute is the default route of any endpoint.
	windowsDefaultRoute = "0.0.0.0/0"
	// credentialsEndpointRoute is the route of credentials endpoint for accessing task iam roles/task metadata.
	credentialsEndpointRoute = "169.254.170.2/32"
	// imdsEndpointIPAddress is the IP address of the endpoint for accessing IMDS.
	imdsEndpointIPAddress = "169.254.169.254/32"
	ecsBridgeSubnet       = "169.254.172.0/22"

	// Starting with CNI plugin v0.8.0 (this PR https://github.com/containernetworking/cni/pull/698)
	// NetworkName has to be non-empty field for network config.
	// We do not actually make use of the field, hence passing in a placeholder string to fulfill the API spec
	defaultNetworkName = "network-name"
	// DefaultENIName is the name of eni interface name in the container namespace
	DefaultENIName = "eth0"
)

var (
	// Values for creating backoff while retrying setupNS.
	// These have not been made constant so that we can inject different values for unit tests.
	setupNSBackoffMin      = time.Second * 4
	setupNSBackoffMax      = time.Minute
	setupNSBackoffJitter   = 0.2
	setupNSBackoffMultiple = 2.0
	setupNSMaxRetryCount   = 5
)

// VPCENIPluginConfig contains all the information required to invoke the vpc-eni plugin.
type VPCENIPluginConfig struct {
	// Type is the cni plugin name.
	Type string `json:"type,omitempty"`
	// CNIVersion is the cni spec version to use.
	CNIVersion string `json:"cniVersion,omitempty"`
	// DNS is used to pass DNS information to the plugin.
	DNS types.DNS `json:"dns"`

	// ENIName is the name of the eni on the instance.
	ENIName string `json:"eniName"`
	// ENIMACAddress is the MAC address of the eni.
	ENIMACAddress string `json:"eniMACAddress"`
	// ENIIPAddresses is the is the ipv4 of eni.
	ENIIPAddresses []string `json:"eniIPAddresses"`
	// GatewayIPAddresses specifies the IPv4 address of the subnet gateway for the eni.
	GatewayIPAddresses []string `json:"gatewayIPAddresses"`
	// UseExistingNetwork specifies if existing network should be used instead of creating a new one.
	UseExistingNetwork bool `json:"useExistingNetwork"`
	// BlockIMDS specifies if the IMDS should be blocked for the created endpoint.
	BlockIMDS    bool               `json:"blockInstanceMetadata"`
	PortMappings []PortMappingEntry `json:"portMappings"`
}

// VPCBridgePluginConfig contains all the information required to invoke vpc-bridge plugin.
type VPCBridgePluginConfig struct {
	// Type is the cni plugin name.
	Type string `json:"type,omitempty"`
	// CNIVersion is the cni spec version to use.
	CNIVersion string `json:"cniVersion,omitempty"`
	// DNS is used to pass DNS information to the plugin.
	DNS types.DNS `json:"dns"`

	NetworkType   string             `json:"networkType"`
	NetworkSubnet string             `json:"networkSubnet"`
	PortMappings  []PortMappingEntry `json:"portMappings"`
	// EniName is the name of the ENI to use for the bridge
	ENIName string `json:"eniName"`
	// EniMacAddress is the address of the ENI
	ENIMACAddress string `json:"eniMacAddress"`
	// EniIPAddresses are the IP addresses assigned to the ENI
	ENIIPAddresses []string `json:"eniIPAddresses"`
	// VPCCIDRs are the CIDR blocks assigned to the VPC
	VPCCIDRs []string `json:"vpcCIDRs"`
	// BridgeType is one of "L2" or "L3", defaults to "L3"
	BridgeType string `json:"bridgeType"`
	// BridgeNetNSPath is the namespace that the vpc-bridge
	// will be created in, "" by default
	BridgeNetNSPath string `json:"bridgeNetNSPath"`
	// IPAddresses are the addresses that will be assigned to the
	// other end of the veth pair connected to the bridge
	IPAddresses []string `json:"ipAddresses"`
	// GatewayIPAddres is the address of the gateway, likely the
	// gateway of the subnet that this ENI exists in
	GatewayIPAddress string `json:"gatewayIPAddress"`
	// InterfaceType is one of "veth" or "tap", defaults to "veth"
	InterfaceType string `json:"interfaceType"`
	// TapUserID is the ID of the linux user that owns the tap interface
	TapUserID      string   `json:"tapUserID"`
	BlockIMDS      bool     `json:"blockInstanceMetadata"`
	RoutesToAdd    []string `json:"routesToAdd"`
	RoutesToDelete []string `json:"routesToDelete"`
}

type PortMappingEntry struct {
	Protocol      string `json:"protocol"`
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort"`
}
