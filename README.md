# Enables k8s containers to access devices (linux device drivers) available on nodes

## Motivation

In the IoT world, interaction with the external environment is the reason of existence.
This interaction is done by acquiring data about the environment and, possibly, actuating to achieve the desired objective, with complexity ranging from a simple thermostat to a very complex industrial process control (e.g. chemical plant). In more practical terms, the main CPU interacts directly with those sensors and actuators and the OS (Linux in our case) provides an abstract view in the form of device drivers.
Even though the container runtime allows direct access to device drivers, containers running on Kubernetes in the cloud are not expected to do so since hardware independence is a very useful characteristic to enhance mobility.
Kubernetes primarily manages CPU, memory, storage, and network, while leaving other resources unmanaged.
In IoT environments, applications can have direct access to sensors and actuators, either directly by interfacing with a device driver on the kernel (e.g. digital I/O pins, temperature sensors, analog inputs, microphones, audio output, video cameras) or indirectly through hardware interfaces (like serial ports, I2C, SPI, bluetooth, LoRa, USB and others).
Controlled access to these devices is essential to enable a container-based IoT solution. Smarter-device-manager allows containers to have direct access to host devices in a secure way.

## Usage Model

The smarter-device-manager starts by reading a YAML configuration file. This configuration file describes, using regular expressions, the files that identify each device that is to be exported and how many access can be done simultaneously. For example, the configuration below finds every V4L device (cameras, video tuners, etc...) available on the host node (/dev/video0, /dev/video1, etc), and adds them as resources (smarter-devices/video0, smarter-devices/video1, etc) that allow up to 10 simulatenous accesses (up to 10 containers can request access to those devices simultaneously). 
```
- devicematch: ^video[0-9]*$
  nummaxdevices: 10
```

Devices in subdirectories have the slash replaced with underscore in the
resource name, due to kubernetes naming restrictions: e.g. `/dev/net/tun`
becomes `smarter-devices/net_tun`.

The default config file provided will enable most of the devices available on a Raspberry Pi (vers 1-4) or equivalent boards. I2C, SPI, video devices, sound and others would be enabled. The config file can be replaced using a configmap to enable or disable access to different devices, like accelerators, GPUs, etc.

The node will show the devices it recognizes as resources in the node object in Kubernetes. The example below shows a raspberry PI.
```
kubectl describe node pike5


Name:               pike5
Roles:              <none>
Labels:             beta.kubernetes.io/arch=arm
                    beta.kubernetes.io/os=linux
                    fake-image-generator-rpi=enabled
                    smarter-device-manager=enabled
Annotations:        node.alpha.kubernetes.io/ttl: 0
CreationTimestamp:  Mon, 02 Dec 2019 09:22:56 -0600
Taints:             <none>
Unschedulable:      false
Lease:
  HolderIdentity:  <unset>
  AcquireTime:     <unset>
  RenewTime:       <unset>
Conditions:
  Type             Status  LastHeartbeatTime                 LastTransitionTime                Reason                       Message
  ----             ------  -----------------                 ------------------                ------                       -------
  MemoryPressure   False   Thu, 16 Jan 2020 08:20:06 -0600   Mon, 02 Dec 2019 09:22:56 -0600   KubeletHasSufficientMemory   kubelet has sufficient memory available
  DiskPressure     False   Thu, 16 Jan 2020 08:20:06 -0600   Wed, 04 Dec 2019 09:47:08 -0600   KubeletHasNoDiskPressure     kubelet has no disk pressure
  PIDPressure      False   Thu, 16 Jan 2020 08:20:06 -0600   Mon, 02 Dec 2019 09:22:56 -0600   KubeletHasSufficientPID      kubelet has sufficient PID available
  Ready            True    Thu, 16 Jan 2020 08:20:06 -0600   Mon, 16 Dec 2019 14:58:05 -0600   KubeletReady                 kubelet is posting ready status. AppArmor enabled
Addresses:
  InternalIP:  XXX.XXX.XXX.XXX
  Hostname:    pike5
Capacity:
  cpu:                        4
  ephemeral-storage:          14999512Ki
  memory:                     873348Ki
  pods:                       110
  smarter-devices/gpiochip0:  10
  smarter-devices/gpiochip1:  10
  smarter-devices/gpiochip2:  10
  smarter-devices/gpiomem:    10
  smarter-devices/i2c-1:      10
  smarter-devices/snd:        10
  smarter-devices/vchiq:      10
  smarter-devices/vcs:        0
  smarter-devices/vcsm:       10
  smarter-devices/vcsm-cma:   0
  smarter-devices/video10:    0
  smarter-devices/video11:    0
  smarter-devices/video12:    0
  smarter-devices/video4:     10
Allocatable:
  cpu:                        4
  ephemeral-storage:          13823550237
  memory:                     770948Ki
  pods:                       110
  smarter-devices/gpiochip0:  10
  smarter-devices/gpiochip1:  10
  smarter-devices/gpiochip2:  10
  smarter-devices/gpiomem:    10
  smarter-devices/i2c-1:      10
  smarter-devices/snd:        10
  smarter-devices/vchiq:      10
  smarter-devices/vcs:        0
  smarter-devices/vcsm:       10
  smarter-devices/vcsm-cma:   0
  smarter-devices/video10:    0
  smarter-devices/video11:    0
  smarter-devices/video12:    0
  smarter-devices/video4:     10
System Info:
  Machine ID:                 XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
  System UUID:                XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
  Boot ID:                    XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
  Kernel Version:             5.3.0-1014-raspi2
  OS Image:                   Ubuntu 19.10
  Operating System:           linux
  Architecture:               arm
  Container Runtime Version:  docker://19.3.2
  Kubelet Version:            v1.13.5
  Kube-Proxy Version:         v1.13.5
Non-terminated Pods:          (5 in total)
  Namespace                   Name                              CPU Requests  CPU Limits  Memory Requests  Memory Limits  AGE
  ---------                   ----                              ------------  ----------  ---------------  -------------  ---
  argus                       smarter-device-manager-gdmjk      10m (0%)      100m (2%)   15Mi (1%)        15Mi (1%)      43d
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource                   Requests     Limits
  --------                   --------     ------
  cpu                        560m (14%)   850m (21%)
  memory                     365Mi (48%)  365Mi (48%)
  ephemeral-storage          0 (0%)       0 (0%)
  smarter-devices/gpiochip0  0            0
  smarter-devices/gpiochip1  0            0
  smarter-devices/gpiochip2  0            0
  smarter-devices/gpiomem    0            0
  smarter-devices/i2c-1      0            0
  smarter-devices/snd        0            0
  smarter-devices/vchiq      1            1
  smarter-devices/vcs        0            0
  smarter-devices/vcsm       1            1
  smarter-devices/vcsm-cma   0            0
  smarter-devices/video10    0            0
  smarter-devices/video11    0            0
  smarter-devices/video12    0            0
  smarter-devices/video4     2            2
Events:                      <none>
```

## System Architecture

The smarter-device-manager is a container that, when deployed, reads the /dev directory and, based on the provided configuration file located at "/root/config/conf.yaml", identifies which devices it can export. The container then uses the Kubernetes kubelet device plugin interface to inform the kubelet that those devices are available. Kubelet will use the plugin interface to ask the smarter-device-manager how to enable access to each device when a pod requests access to that device. Smarter-device-manager uses the "--device" option of the OCI to add that device to the container /dev directory and adds that device to the device cgroup so the container.

More than one smarter-device-manager can be used in a single node if required if they enable different devices. 

## Enabling Access

A few examples of yaml files are provided that enable the smarter-device-manager to be deployed in a node. The file smarter-device-management-pod-<>.yaml deploys a single pod on a node; this setup is useful for testing. The file smarter-device-manager-<>.yaml provides a deamonSet configuration that enables pods to be deployed in any node that contains the "smarter-device-manager=enabled" label. The following command inserts the daemonSet in Kubernetes. Use the k8s for k8s/k3s/k0s unless using k3s version lower than 1.18. K3s smaller then 1.18 put the unix sockets for the device plugin in different directories on the node so the \*-k3s.yaml files should be used on k3s for those versions.

```
kubectl apply -f smarter-device-manager.yaml
```
and the following command deploys a smarter-device-manager pod on a node (pike5)
```
kubectl label node pike5 smarter-device-manager=enabled
```
The following command should show the node resources in a similar form as shown in previous example:
```
kubectl describe node pike5
```

## k3s 

K3s < 1.18 stores the plugin interface in a different directory than k8s and so it needs a different yaml file to enable smarter-device-manager to communicate correctly with k3s agent. So use the smart-device-manager-k3s yaml files on this reposistor for k3s < 1.18.

## Using helm

A helm chart that install smarter-device-manager configured for SMARTER is available at chart directory
```
helm install smarter-device-manager chart
```
