//go:build windows

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

package stats

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api/task"
	"github.com/aws/amazon-ecs-agent/agent/stats/resolver"

	dockerstats "github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

// Making it visible for unit testing
var execCommand = exec.Command

type StatsTask struct {
	*statsTaskCommon
}

func newStatsTaskContainer(taskARN string, containerPID string, numberOfContainers int,
	resolver resolver.ContainerMetadataResolver, publishInterval time.Duration, taskENIs task.TaskENIs) (*StatsTask, error) {
	ctx, cancel := context.WithCancel(context.Background())

	devices := make([]string, len(taskENIs))
	for index, device := range taskENIs {
		devices[index] = device.LinkName
	}

	return &StatsTask{
		statsTaskCommon: &statsTaskCommon{
			TaskMetadata: &TaskMetadata{
				TaskArn:          taskARN,
				ContainerPID:     containerPID,
				DeviceName:       devices,
				NumberContainers: numberOfContainers,
			},
			Ctx:                   ctx,
			Cancel:                cancel,
			Resolver:              resolver,
			metricPublishInterval: publishInterval,
		},
	}, nil
}

func (taskStat *StatsTask) retrieveNetworkStatistics() (map[string]dockerstats.NetworkStats, error) {
	if len(taskStat.TaskMetadata.DeviceName) == 0 {
		return nil, errors.Errorf("unable to find any device name associated with the task %s", taskStat.TaskMetadata.TaskArn)
	}

	networkStats := make(map[string]dockerstats.NetworkStats, len(taskStat.TaskMetadata.DeviceName))
	for _, device := range taskStat.TaskMetadata.DeviceName {
		networkAdaptorStatistics, err := taskStat.getNetworkAdaptorStatistics(device)
		if err != nil {
			return nil, err
		}
		networkStats[device] = *networkAdaptorStatistics
	}

	return networkStats, nil
}

// getNetworkAdaptorStatistics returns the network statistics per container for the given network interface.
func (taskStat *StatsTask) getNetworkAdaptorStatistics(device string) (*dockerstats.NetworkStats, error) {
	// Ref: https://docs.microsoft.com/en-us/powershell/module/netadapter/get-netadapterstatistics?view=windowsserver2019-ps
	// The Get-NetAdapterStatistics cmdlet gets networking statistics from a network adapter.
	// The statistics include broadcast, multicast, discards, and errors.
	cmd := "Get-NetAdapterStatistics -Name \"" + device + "\" | Format-List -Property *"
	out, err := execCommand("powershell", "-Command", cmd).CombinedOutput()

	if err != nil {
		return nil, errors.Wrapf(err, "failed to run Get-NetAdapterStatistics for %s", device)
	}
	str := string(out)

	lines := strings.Split(str, "\n")
	m := make(map[string]string)
	for _, line := range lines {
		// populate all the network metrics in a map
		kv := strings.Split(line, ":")
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		m[key] = value
	}

	receivedBroadcastPackets, err := strconv.ParseUint(m["ReceivedBroadcastPackets"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : ReceivedBroadcastPackets", device)
	}

	receivedMulticastPackets, err := strconv.ParseUint(m["ReceivedMulticastPackets"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : ReceivedMulticastPackets", device)
	}

	receivedUnicastPackets, err := strconv.ParseUint(m["ReceivedUnicastPackets"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : ReceivedUnicastPackets", device)
	}

	sentBroadcastPackets, err := strconv.ParseUint(m["SentBroadcastPackets"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : SentBroadcastPackets", device)
	}

	sentMulticastPackets, err := strconv.ParseUint(m["SentMulticastPackets"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : field SentMulticastPackets", device)
	}

	sentUnicastPackets, err := strconv.ParseUint(m["SentUnicastPackets"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : field SentUnicastPackets", device)
	}

	receivedBytes, err := strconv.ParseUint(m["ReceivedBytes"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : ReceivedBytes", device)
	}

	receivedPacketErrors, err := strconv.ParseUint(m["ReceivedPacketErrors"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : ReceivedPacketErrors", device)
	}

	receivedDiscardedPackets, err := strconv.ParseUint(m["ReceivedDiscardedPackets"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : ReceivedDiscardedPackets", device)
	}

	sentBytes, err := strconv.ParseUint(m["SentBytes"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : SentBytes", device)
	}

	outboundPacketErrors, err := strconv.ParseUint(m["OutboundPacketErrors"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : OutboundPacketErrors", device)
	}

	outboundDiscardedPackets, err := strconv.ParseUint(m["OutboundDiscardedPackets"], 10, 64)
	if err != nil {
		return nil, errors.Errorf("failed to parse network stats for %s : OutboundDiscardedPackets", device)
	}

	numberOfContainers := uint64(taskStat.TaskMetadata.NumberContainers)

	return &dockerstats.NetworkStats{
		RxBytes:   receivedBytes / numberOfContainers,
		RxPackets: (receivedBroadcastPackets + receivedMulticastPackets + receivedUnicastPackets) / numberOfContainers,
		RxErrors:  receivedPacketErrors / numberOfContainers,
		RxDropped: receivedDiscardedPackets / numberOfContainers,
		TxBytes:   sentBytes / numberOfContainers,
		TxPackets: (sentBroadcastPackets + sentMulticastPackets + sentUnicastPackets) / numberOfContainers,
		TxErrors:  outboundPacketErrors / numberOfContainers,
		TxDropped: outboundDiscardedPackets / numberOfContainers,
	}, nil
}
