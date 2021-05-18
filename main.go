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
        pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
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

func readDevDirectory(dirToList string, allowedRecursions uint8) (files []string, err error) {
        var foundFiles []string

        fType, err := os.Stat(dirToList)
        if err != nil {
                return nil, err
        }

        if !fType.IsDir()  {
                return nil, nil
        }

        f, err := os.Open(dirToList)
        if err != nil {
                return nil, err
        }
        files, err = f.Readdirnames(-1)
        if err != nil {
                f.Close()
                return nil, err
        }
        f.Close()
        for _, subDir := range files {
                foundFiles = append(foundFiles, subDir)
                if allowedRecursions > 0 {
                        filesDir, err := readDevDirectory(dirToList+"/"+subDir,allowedRecursions-1)
                        if err == nil {
                                for _, fileName := range filesDir {
                                        foundFiles = append(foundFiles, subDir+"/"+fileName)
                                }
                        }
                }
        }

        return foundFiles, nil
}

func sanitizeName(path string) string {
        return strings.Replace(path, "/", "_" ,-1)
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
	ExistingDevices, err := readDevDirectory("/dev",10)
	if err != nil {
		glog.Errorf(err.Error())
		os.Exit(1)
	}

	ExistingDevicesSys, err := readDevDirectory("/sys/devices",0)
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
                                        deviceSafeName := sanitizeName(deviceToCreate)
                                        newDevice.deviceType = deviceFileType
                                        newDevice.deviceName = "smarter-devices/" + deviceSafeName
                                        newDevice.socketName = pluginapi.DevicePluginPath + "smarter-" + deviceSafeName + ".sock"
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
			for id, _ := range listDevicesAvailable {
                                switch listDevicesAvailable[id].deviceType {
                                case deviceFileType :
                                        listDevicesAvailable[id].devicePluginSmarter = NewSmarterDevicePlugin(listDevicesAvailable[id].numDevices, listDevicesAvailable[id].deviceFile, listDevicesAvailable[id].deviceName, listDevicesAvailable[id].socketName)
                                        if err = listDevicesAvailable[id].devicePluginSmarter.Serve(); err != nil {
                                                glog.V(0).Info("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
                                                break
                                        }
                                case nvidiaSysType :
                                        listDevicesAvailable[id].devicePluginNvidia = NewNvidiaDevicePlugin(listDevicesAvailable[id].numDevices, listDevicesAvailable[id].deviceName,"NVIDIA_VISIBLE_DEVICES", listDevicesAvailable[id].socketName, listDevicesAvailable[id].deviceId)
                                        if err = listDevicesAvailable[id].devicePluginNvidia.Serve(); err != nil {
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
                                        glog.V(0).Info("Stopping device ", devicesInUse.deviceName)
                                        switch devicesInUse.deviceType {
                                        case deviceFileType :
				                glog.V(0).Info("Smarter device type")
                                                if devicesInUse.devicePluginSmarter != nil {
				                        glog.V(0).Info("Stopping device")
                                                        devicesInUse.devicePluginSmarter.Stop()
                                                }
                                        case nvidiaSysType :
				                glog.V(0).Info("Nvidia device type")
                                                if devicesInUse.devicePluginNvidia != nil {
				                        glog.V(0).Info("Stopping device")
                                                        devicesInUse.devicePluginNvidia.Stop()
                                                }
                                        }
				}
				break L
			}
		}
	}
}
