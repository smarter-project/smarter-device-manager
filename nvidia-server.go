// Copyright (c) 2019, Arm Ltd

package main

import (
        "flag"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

var passDeviceSpecs = flag.Bool("pass-device-specs", false, "pass the list of DeviceSpecs to the kubelet on Allocate()")

// NvidiaDevicePlugin implements the Kubernetes device plugin API
type NvidiaDevicePlugin struct {
	devs         []*pluginapi.Device
	socket       string
	resourceName   string
	allocateEnvvar string
        id string


	stop   chan interface{}
	health chan *pluginapi.Device

	server *grpc.Server
}

// NewNvidiaDevicePlugin returns an initialized NvidiaDevicePlugin
func NewNvidiaDevicePlugin(nDevices uint, resourceName string, allocateEnvvar string, socket string, id string) *NvidiaDevicePlugin {
	return &NvidiaDevicePlugin{
                devs:            getDevices(nDevices),
		resourceName:    resourceName,
		allocateEnvvar:  allocateEnvvar,
		socket:          socket,
		id:              id,

                stop:   make(chan interface{}),
                health: make(chan *pluginapi.Device),
	}
}

// dial establishes the gRPC communication with the registered device plugin.
func dialNvidia(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

// Start the gRPC server of the device plugin
func (m *NvidiaDevicePlugin) Start() error {
	glog.V(0).Info("Initializing nvidia device manager")
	err := m.cleanup()
	if err != nil {
		return err
	}

	glog.V(0).Info("Opening nvidia device manager socket ", m.socket)
	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}
	glog.V(0).Info("Socket opened nvidia device manager")

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(m.server, m)
	glog.V(0).Info("gRPC server registered")

	go m.server.Serve(sock)
	glog.V(0).Info("gRPC server running on socket")

	// Wait for server to start by launching a blocking connexion
	conn, err := dialNvidia(m.socket, 60*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	glog.V(0).Info("gRPC Dial OK")

	go m.healthcheck()

	return nil
}

// Stop the gRPC server
func (m *NvidiaDevicePlugin) Stop() error {
	if m.server == nil {
		return nil
	}

	m.server.Stop()
	m.server = nil
	close(m.stop)

	return m.cleanup()
}

// Register the device plugin for the given resourceName with Kubelet.
func (m *NvidiaDevicePlugin) Register(kubeletEndpoint, resourceName string) error {
	conn, err := dialNvidia(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(m.socket),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// ListAndWatch lists devices and update that list according to the health status
func (m *NvidiaDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})

	for {
		select {
		case <-m.stop:
			return nil
		case d := <-m.health:
			// FIXME: there is no way to recover from the Unhealthy state.
			d.Health = pluginapi.Unhealthy
			s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
		}
	}
}

func (m *NvidiaDevicePlugin) unhealthy(dev *pluginapi.Device) {
	m.health <- dev
}

// Allocate which return list of devices.
func (m *NvidiaDevicePlugin) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
                //for _, id := range req.DevicesIDs {
		//	if !m.deviceExists(id) {
		//		return nil, fmt.Errorf("invalid allocation request for '%s': unknown device: %s", m.resourceName, id)
		//	}
		//

		response := pluginapi.ContainerAllocateResponse{
			Envs: map[string]string{
				m.allocateEnvvar: m.id,
			},
		}
		if *passDeviceSpecs {
			response.Devices = m.apiDeviceSpecs(req.DevicesIDs)
		}

		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}

	return &responses, nil
}

func (m *NvidiaDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (m *NvidiaDevicePlugin) cleanup() error {
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (m *NvidiaDevicePlugin) healthcheck() {
	disableHealthChecks := strings.ToLower(os.Getenv(envDisableHealthChecks))
	if disableHealthChecks == "all" {
		disableHealthChecks = allHealthChecks
	}

	_, cancel := context.WithCancel(context.Background())

	var xids chan *pluginapi.Device
	if !strings.Contains(disableHealthChecks, "xids") {
		xids = make(chan *pluginapi.Device)
	}

	for {
		select {
		case <-m.stop:
			cancel()
			return
		case dev := <-xids:
			m.unhealthy(dev)
		}
	}
}

// Serve starts the gRPC server and register the device plugin to Kubelet
func (m *NvidiaDevicePlugin) Serve() error {
	err := m.Start()
	if err != nil {
		glog.Errorf("Could not start device plugin: %s", err)
		return err
	}
	glog.V(0).Info("Starting to serve on", m.socket)

	err = m.Register(pluginapi.KubeletSocket, m.resourceName)
	if err != nil {
		glog.Errorf("Could not register device plugin: %s", err)
		m.Stop()
		return err
	}
	glog.V(0).Info("Registered device plugin with Kubelet")

	return nil
}

func (m *NvidiaDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (m *NvidiaDevicePlugin) apiDeviceSpecs(filter []string) []*pluginapi.DeviceSpec {
	var specs []*pluginapi.DeviceSpec

	paths := []string{
		"/dev/nvidiactl",
		"/dev/nvidia-uvm",
		"/dev/nvidia-uvm-tools",
		"/dev/nvidia-modeset",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			spec := &pluginapi.DeviceSpec{
				ContainerPath: p,
				HostPath:      p,
				Permissions:   "rw",
			}
			specs = append(specs, spec)
		}
	}

//	for _, d := range m.devs {
//		for _, id := range filter {
//			if d.ID == id {
//				spec := &pluginapi.DeviceSpec{
//					ContainerPath: d.Path,
//					HostPath:      d.Path,
//					Permissions:   "rw",
//				}
//				specs = append(specs, spec)
//			}
//		}
//	}

	return specs
}
