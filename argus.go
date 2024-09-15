// Copyright (c) 2019, Arm Ltd

package main

import (
        pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"strconv"
)

func getDevices(n uint) []*pluginapi.Device {
	var devs []*pluginapi.Device
	for i := 0; i < n; i++ {
		devs = append(devs, &pluginapi.Device{
			ID:     strconv.Itoa(i),
			Health: pluginapi.Healthy,
		})
	}

	return devs
}

func deviceExists(devs []*pluginapi.Device, id string) bool {
	for _, d := range devs {
		if d.ID == id {
			return true
		}
	}
	return false
}
