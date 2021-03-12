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
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/logger"
	"github.com/cihub/seelog"
	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/pkg/errors"
)

const (
	currentCNISpec = "0.3.1"
	// ECSCNIVersion, ECSCNIGitHash, VPCCNIGitHash needs to be updated every time CNI plugin is updated
	currentECSCNIVersion      = "2020.09.0"
	currentECSCNIGitHash      = "55b2ae77ee0bf22321b14f2d4ebbcc04f77322e1"
	currentVPCCNIGitHash      = "a21d3a41f922e14c19387713df66be3e4ee1e1f6"
	vpcCNIPluginInterfaceType = "vlan"
)

// CNIClient defines the method of setting/cleaning up container namespace
type CNIClient interface {
	// Version returns the version of the plugin
	Version(string) (string, error)
	// Capabilities returns the capabilities supported by a plugin
	Capabilities(string) ([]string, error)
	// SetupNS sets up the namespace of container
	SetupNS(context.Context, *Config, time.Duration) (*current.Result, error)
	// CleanupNS cleans up the container namespace
	CleanupNS(context.Context, *Config, time.Duration) error
	// ReleaseIPResource marks the ip available in the ipam db
	ReleaseIPResource(context.Context, *Config, time.Duration) error
}

// cniClient is the client to call plugin and setup the network
type cniClient struct {
	pluginsPath string
	libcni      libcni.CNI
}

// NewClient creates a client of ecscni which is used to invoke the plugin
func NewClient(pluginsPath string) CNIClient {
	libcniConfig := &libcni.CNIConfig{
		Path: []string{pluginsPath},
	}

	cniClient := &cniClient{
		pluginsPath: pluginsPath,
		libcni:      libcniConfig,
	}
	cniClient.init()
	return cniClient
}

func (client *cniClient) init() {
	// Set environment variables for CNI plugins.
	os.Setenv("ECS_CNI_LOGLEVEL", logger.GetLevel())
	os.Setenv("VPC_CNI_LOG_LEVEL", logger.GetLevel())
	os.Setenv("VPC_CNI_LOG_FILE", vpcCNIPluginPath)
}

// SetupNS sets up the network namespace of a task by invoking the given CNI network configurations.
// It returns the result of the bridge plugin invocation as that result is used to parse the IPv4
// address allocated to the veth device attached to the task by the task engine.
func (client *cniClient) SetupNS(
	ctx context.Context,
	cfg *Config,
	timeout time.Duration) (*current.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.setupNS(ctx, cfg)
}

// CleanupNS will clean up the container namespace, including remove the veth
// pair and stop the dhclient
func (client *cniClient) CleanupNS(
	ctx context.Context,
	cfg *Config,
	timeout time.Duration) error {

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return client.cleanupNS(ctx, cfg)
}

// cleanupNS is called by CleanupNS to cleanup the task namespace by invoking DEL for given CNI configurations
func (client *cniClient) cleanupNS(ctx context.Context, cfg *Config) error {
	seelog.Debugf("[ECSCNI] Cleaning up the container namespace %s", cfg.ContainerID)

	runtimeConfig := libcni.RuntimeConf{
		ContainerID: cfg.ContainerID,
		NetNS:       cfg.ContainerNetNS,
	}

	// Execute all CNI network configurations serially, in the reverse order.
	for i := len(cfg.NetworkConfigs) - 1; i >= 0; i-- {
		networkConfig := cfg.NetworkConfigs[i]
		cniNetworkConfig := networkConfig.CNINetworkConfig
		seelog.Debugf("[ECSCNI] Deleting network %s type %s in the container namespace %s",
			cniNetworkConfig.Network.Name,
			cniNetworkConfig.Network.Type,
			cfg.ContainerID)
		runtimeConfig.IfName = networkConfig.IfName
		err := client.libcni.DelNetwork(ctx, cniNetworkConfig, &runtimeConfig)
		if err != nil {
			return errors.Wrap(err, "delete network failed")
		}

		seelog.Debugf("[ECSCNI] Completed deleting network %s type %s in the container namespace %s",
			cniNetworkConfig.Network.Name,
			cniNetworkConfig.Network.Type,
			cfg.ContainerID)
	}

	seelog.Debugf("[ECSCNI] Completed cleaning up the container namespace %s", cfg.ContainerID)

	return nil
}

// Version returns the version of the plugin
func (client *cniClient) Version(name string) (string, error) {
	file := filepath.Join(client.pluginsPath, name)

	// Check if the plugin file exists before executing it
	_, err := os.Stat(file)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(file, versionCommand)
	versionInfo, err := cmd.Output()
	if err != nil {
		return "", err
	}

	version := &cniPluginVersion{}
	// For Linux, versionInfo is of the format
	// {"version":"2017.06.0","dirty":true,"gitShortHash":"226db36"}
	// For Windows, it is of the format
	// {"version":"2017.06.0","gitShortHash":"226db36","built":"2048-08-16T12:10:14-08:00"}
	// Unmarshal this
	err = json.Unmarshal(versionInfo, version)
	if err != nil {
		return "", errors.Wrapf(err, "ecscni: unmarshal version from string: %s", versionInfo)
	}

	return version.str(), nil
}

// Capabilities returns the capabilities supported by a plugin
func (client *cniClient) Capabilities(name string) ([]string, error) {
	file := filepath.Join(client.pluginsPath, name)

	// Check if the plugin file exists before executing it
	_, err := os.Stat(file)
	if err != nil {
		return nil, errors.Wrapf(err, "ecscni: unable to describe file info for '%s'", file)
	}

	cmd := exec.Command(file, capabilitiesCommand)
	capabilitiesInfo, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "ecscni: failed invoking capabilities command for '%s'", name)
	}

	capabilities := &struct {
		Capabilities []string `json:"capabilities"`
	}{}
	err = json.Unmarshal(capabilitiesInfo, capabilities)
	if err != nil {
		return nil, errors.Wrapf(err, "ecscni: failed to unmarshal capabilities for '%s' from string: %s", name, capabilitiesInfo)
	}

	return capabilities.Capabilities, nil
}
