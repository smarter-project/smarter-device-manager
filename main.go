// Copyright (c) 2019, Arm Ltd

package main

import (
	"flag"
	"fmt"
	"strings"
	"os"
	"regexp"
	"syscall"
        "io/ioutil"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

var confFileName string

const (
        deviceFileType uint = 0
        nvidiaSysType uint = 1
)

type DeviceInstance struct {
	devicePluginSmarter *SmarterDevicePlugin
	devicePluginNvidia *NvidiaDevicePlugin

	deviceName string
	socketName string
	deviceFile string
	numDevices uint
        deviceType uint
        deviceId   string
}

type DesiredDevice struct {
	DeviceMatch   string
	NumMaxDevices uint
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: smarter-device-manager\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage
	// NOTE: This next line is key you have to call flag.Parse() for the command line
	// options or "flags" that are defined in the glog module to be picked up.
        flag.StringVar(&confFileName,"config","config/conf.yaml","set the configuration file to use")
	flag.Parse()
}

func readDevDirectory(dirToList string) (files []string, err error) {
	f, err := os.Open(dirToList)
	if err != nil {
		return nil, err
	}
	files, err = f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}

	return files, nil
}

func findDevicesPattern(listDevices []string, pattern string) ([]string,error) {
	var found []string

	for _, file := range listDevices {
		res,err := regexp.MatchString(pattern, file)
                if err != nil {
                        return nil, err
                }
                if res {
                        found = append(found, file)
                }
	}
	return found,nil
}

func main() {
	defer glog.Flush()
	glog.V(0).Info("Loading smarter-device-manager")

	// Setting up the devices to check
        var desiredDevices []DesiredDevice
	glog.V(0).Info("Reading configuration file ",confFileName)
        yamlFile, err := ioutil.ReadFile(confFileName)
        if err != nil {
                glog.Fatal("yamlFile.Get err   #%v ", err)
        }
        err = yaml.Unmarshal(yamlFile, &desiredDevices)
        if err != nil {
                glog.Fatal("Unmarshal: %v", err)
                os.Exit(-1)
        }

	glog.V(0).Info("Reading existing devices on /dev")
	ExistingDevices, err := readDevDirectory("/dev")
	if err != nil {
		glog.Errorf(err.Error())
		os.Exit(1)
	}

	ExistingDevicesSys, err := readDevDirectory("/sys/devices")
	if err != nil {
		glog.Errorf(err.Error())
		os.Exit(1)
	}
	var listDevicesAvailable []DeviceInstance

	for _, deviceToTest := range desiredDevices {
                if deviceToTest.DeviceMatch == "nvidia-gpu" {
                        glog.V(0).Infof("Checking nvidia devices")
                        foundDevices,err := findDevicesPattern(ExistingDevicesSys, "gpu.[0-9]*")
                        if err != nil {
                                glog.Errorf(err.Error())
                                os.Exit(1)
                        }

                        // If found some create the devices entry
                        if len(foundDevices) > 0 {
                                for _, deviceToCreate := range foundDevices {
                                        var newDevice DeviceInstance
                                        deviceId := strings.TrimPrefix(deviceToCreate,"gpu.")
                                        newDevice.deviceName = "smarter-devices/" + "nvidia-gpu" + deviceId
                                        newDevice.deviceId = deviceId
                                        newDevice.socketName = pluginapi.DevicePluginPath + "smarter-nvidia-gpu" + deviceId + ".sock"
                                        newDevice.deviceFile = deviceId
                                        newDevice.numDevices = deviceToTest.NumMaxDevices
                                        newDevice.deviceType = nvidiaSysType
                                        listDevicesAvailable = append(listDevicesAvailable, newDevice)
                                        glog.V(0).Infof("Creating device %s socket and %s name for %s",newDevice.deviceName,newDevice.deviceFile,deviceToTest.DeviceMatch)
                                }
                        }
                } else {
                        glog.V(0).Infof("Checking devices %s on /dev",deviceToTest.DeviceMatch)
                        foundDevices,err := findDevicesPattern(ExistingDevices, deviceToTest.DeviceMatch)
                        if err != nil {
                                glog.Errorf(err.Error())
                                os.Exit(1)
                        }

                        // If found some create the devices entry
                        if len(foundDevices) > 0 {
                                for _, deviceToCreate := range foundDevices {
                                        var newDevice DeviceInstance
                                        newDevice.deviceType = deviceFileType
                                        newDevice.deviceName = "smarter-devices/" + deviceToCreate
                                        newDevice.socketName = pluginapi.DevicePluginPath + "smarter-" + deviceToCreate + ".sock"
                                        newDevice.deviceFile = "/dev/" + deviceToCreate
                                        newDevice.numDevices = deviceToTest.NumMaxDevices
                                        listDevicesAvailable = append(listDevicesAvailable, newDevice)
                                        glog.V(0).Infof("Creating device %s socket and %s name for %s",newDevice.deviceName,newDevice.deviceFile,deviceToTest.DeviceMatch)
                                }
                        }
                }
	}

	glog.V(0).Info("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		glog.V(0).Info("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	glog.V(0).Info("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true

L:
	for {
		if restart {
			for _, devicesInUse := range listDevicesAvailable {
                                switch devicesInUse.deviceType {
                                case deviceFileType :
                                        if devicesInUse.devicePluginSmarter != nil {
                                                devicesInUse.devicePluginSmarter.Stop()
                                        }
                                case nvidiaSysType :
                                        if devicesInUse.devicePluginNvidia != nil {
                                                devicesInUse.devicePluginNvidia.Stop()
                                        }
                                }
			}

			var err error
			for _, devicesInUse := range listDevicesAvailable {
                                switch devicesInUse.deviceType {
                                case deviceFileType :
                                        devicesInUse.devicePluginSmarter = NewSmarterDevicePlugin(devicesInUse.numDevices, devicesInUse.deviceFile, devicesInUse.deviceName, devicesInUse.socketName)
                                        if err = devicesInUse.devicePluginSmarter.Serve(); err != nil {
                                                glog.V(0).Info("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
                                                break
                                        }
                                case nvidiaSysType :
                                        devicesInUse.devicePluginNvidia = NewNvidiaDevicePlugin(devicesInUse.deviceName,"NVIDIA_VISIBLE_DEVICES", devicesInUse.socketName, devicesInUse.deviceId)
                                        if err = devicesInUse.devicePluginNvidia.Serve(); err != nil {
                                                glog.V(0).Info("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
                                                break
                                        }
                                }
			}
			if err != nil {
				continue
			}

			restart = false
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				glog.V(0).Infof("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				restart = true
			}

		case err := <-watcher.Errors:
			glog.V(0).Infof("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				glog.V(0).Info("Received SIGHUP, restarting.")
				restart = true
			default:
				glog.V(0).Infof("Received signal \"%v\", shutting down.", s)
				for _, devicesInUse := range listDevicesAvailable {
                                        switch devicesInUse.deviceType {
                                        case deviceFileType :
                                                if devicesInUse.devicePluginSmarter != nil {
                                                        devicesInUse.devicePluginSmarter.Stop()
                                                }
                                        case nvidiaSysType :
                                                if devicesInUse.devicePluginNvidia != nil {
                                                        devicesInUse.devicePluginNvidia.Stop()
                                                }
                                        }
				}
				break L
			}
		}
	}
}
