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

package statsretriever

import (
	"syscall"
	"unsafe"

	log "github.com/cihub/seelog"
	dockerstats "github.com/docker/docker/api/types"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

type retriever struct {
	funcGetIfTable2Ex func(a ...uintptr) (r1 uintptr, r2 uintptr, lastErr error)
	funcFreeMibTable  func(a ...uintptr) (r1 uintptr, r2 uintptr, lastErr error)
}

func NewStatsRetriever() StatsRetriever {
	log.Debugf("Initializing stats retriever for Windows")
	obj := &retriever{}

	moduleIPHelper := windows.NewLazySystemDLL("iphlpapi.dll")
	procGetIfTable2Ex := moduleIPHelper.NewProc("GetIfTable2Ex")
	procFreeMibTable := moduleIPHelper.NewProc("FreeMibTable")

	obj.funcGetIfTable2Ex = procGetIfTable2Ex.Call
	obj.funcFreeMibTable = procFreeMibTable.Call

	return obj
}

func (retriever *retriever) GetNetworkAdapterStatisticsPerContainer(device string, numberOfContainers uint64) (*dockerstats.NetworkStats, error) {
	log.Debugf("Retrieving the network stats for %s", device)
	allIfaces, err := retriever.getIfTable2Ex(0)
	if err != nil {
		return nil, err
	}
	log.Debugf("Interface table returned by Windows API: %+v", allIfaces)

	for _, iface := range allIfaces {
		alias := windows.UTF16ToString(iface.alias[:])
		if alias == device {
			stats := &dockerstats.NetworkStats{}
			stats.RxBytes = iface.inOctets / numberOfContainers
			stats.RxPackets = (iface.inNUcastPkts + iface.inUcastPkts) / numberOfContainers
			stats.RxErrors = iface.inErrors / numberOfContainers
			stats.RxDropped = iface.inDiscards / numberOfContainers
			stats.TxBytes = iface.outOctets / numberOfContainers
			stats.TxPackets = (iface.outNUcastPkts + iface.outNUcastPkts) / numberOfContainers
			stats.TxErrors = iface.outErrors / numberOfContainers
			stats.TxDropped = iface.outDiscards / numberOfContainers

			return stats, nil
		}
	}

	return nil, errors.Errorf("%s device not found in interface table", device)
}

// getIfTable2Ex function retrieves the MIB-II interface table.
// https://docs.microsoft.com/en-us/windows/desktop/api/netioapi/nf-netioapi-getiftable2ex
func (retriever *retriever) getIfTable2Ex(level mibIfEntryLevel) ([]MibIfRow2, error) {
	var tab *mibIfTable2
	retVal, _, _ := retriever.funcGetIfTable2Ex(uintptr(level), uintptr(unsafe.Pointer(&tab)))
	if retVal != 0 {
		return nil, errors.Errorf("error occured while calling GetIfTable2Ex: %s", syscall.Errno(retVal))
	}

	// Copy the returned values in a new array so we can free the OS allocated memory.
	// This is as per Windows docs- https://learn.microsoft.com/en-us/windows/win32/api/netioapi/nf-netioapi-getiftable2
	table := append(make([]MibIfRow2, 0, tab.numEntries), unsafe.Slice(&tab.table[0], tab.numEntries)...)

	retVal, _, _ = retriever.funcFreeMibTable(uintptr(unsafe.Pointer(tab)))
	if retVal != 0 {
		return nil, errors.Errorf("error occured while calling FreeMibTable: %s", syscall.Errno(retVal))
	}

	return table, nil
}
