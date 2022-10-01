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

package stats

import (
	"context"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api/task"
	"github.com/aws/amazon-ecs-agent/agent/stats/resolver"
	"github.com/aws/amazon-ecs-agent/agent/stats/statsretriever"

	dockerstats "github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

type StatsTask struct {
	*statsTaskCommon
	statsRetriever statsretriever.StatsRetriever
}

func newStatsTaskContainer(taskARN, taskId, containerPID string, numberOfContainers int,
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
		statsRetriever: statsretriever.NewStatsRetriever(),
	}, nil
}

func (taskStat *StatsTask) retrieveNetworkStatistics() (map[string]dockerstats.NetworkStats, error) {
	if len(taskStat.TaskMetadata.DeviceName) == 0 {
		return nil, errors.Errorf("unable to find any device name associated with the task %s", taskStat.TaskMetadata.TaskArn)
	}

	networkStats := make(map[string]dockerstats.NetworkStats, len(taskStat.TaskMetadata.DeviceName))
	for _, device := range taskStat.TaskMetadata.DeviceName {
		numberOfContainers := uint64(taskStat.TaskMetadata.NumberContainers)
		networkAdaptorStatistics, err := taskStat.statsRetriever.GetNetworkAdapterStatisticsPerContainer(device, numberOfContainers)
		if err != nil {
			return nil, err
		}
		networkStats[device] = *networkAdaptorStatistics
	}

	return networkStats, nil
}
