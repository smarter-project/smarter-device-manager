# SMARTER Device Manager

Enables k8s containers to access devices (linux device drivers) available on nodes.

For more information check out https://getsmarter.io

## TL;DR

Assumes that this repository was cloned.

```console
helm install --nsmespace=smarter --create-namespace my-smarter-device-manager charts/smarter-device-manager
```

## Overview

In the IoT world, interaction with the external environment is the reason of existence.
This interaction is done by acquiring data about the environment and, possibly, actuating to achieve the desired objective, with complexity ranging from a simple thermostat to a very complex industrial process control (e.g. chemical plant). In more practical terms, the main CPU interacts directly with those sensors and actuators and the OS (Linux in our case) provides an abstract view in the form of device drivers.
Even though the container runtime allows direct access to device drivers, containers running on Kubernetes in the cloud are not expected to do so since hardware independence is a very useful characteristic to enhance mobility.
Kubernetes primarily manages CPU, memory, storage, and network, while leaving other resources unmanaged.
In IoT environments, applications can have direct access to sensors and actuators, either directly by interfacing with a device driver on the kernel (e.g. digital I/O pins, temperature sensors, analog inputs, microphones, audio output, video cameras) or indirectly through hardware interfaces (like serial ports, I2C, SPI, bluetooth, LoRa, USB and others).
Controlled access to these devices is essential to enable a container-based IoT solution. Smarter-device-manager allows containers to have direct access to host devices in a secure way.

## Values

The configuration.nodeSelector value allows the nodeSelector to be changed in a higher level chart simplyfyng deploying multiple services at the same time; CNI, DNS and device-manager with a single label for example.

## Pre-requisites

- k8s > 1.18 (before this the plugin interface used a different directory which requires a different configuration)
- by default, smarter-device manager uses a node-select to choose which nodes to deploy to, so label your nodes appropriately in order to deploy:
```
kubectl label node mynode01 smarter-device-manager=enabled
```

## Usage Model

The smarter-device-manager starts by reading a YAML configuration file. This configuration file describes, using regular expressions, the files that identify each device that is to be exported and how many access can be done simultaneously. For example, the configuration below finds every V4L device (cameras, video tuners, etc...) available on the host node (/dev/video0, /dev/video1, etc), and adds them as resources (smarter-devices/video0, smarter-devices/video1, etc) that allow up to 10 simulatenous accesses (up to 10 containers can request access to those devices simultaneously). 
```
- devicematch: ^video[0-9]*$
  nummaxdevices: 10
```

If the config value is provided a configMap is generated and smarter-device-manager will use it. The values.yaml file contains two examples, the first is replicated the config that exists on the container and the second enables nitro-enclaves (AWS nitro).

Devices in subdirectories have the slash replaced with underscore in the
resource name, due to kubernetes naming restrictions: e.g. `/dev/net/tun`
becomes `smarter-devices/net_tun`.

The default config file provided will enable most of the devices available on a Raspberry Pi (vers 1-4) or equivalent boards. I2C, SPI, video devices, sound and others would be enabled. The config file can be replaced using a configmap to enable or disable access to different devices, like accelerators, GPUs, etc.

# Uninstalling the Chart

```
helm delete my-smarter-device-manager

```
