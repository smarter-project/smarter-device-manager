// Copyright (c) 2019, ARM

package main

import (
	"github.com/golang/glog"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

func check(err error) {
	if err != nil {
		glog.Errorf(err.Error())
	}
}

func getDevices(n uint) []*pluginapi.Device {
	var devs []*pluginapi.Device
	for i := uint(0); i < n; i++ {
		devs = append(devs, &pluginapi.Device{
			ID:     string(i),
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
