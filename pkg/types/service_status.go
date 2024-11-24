package types

import "time"

type NamespaceGroup struct {
	Namespace string           `json:"namespace"`
	Services  []*ServiceStatus `json:"services"`
}

type EndpointInfo struct {
	PodName string `json:"podName"`
	IP      string `json:"ip"`
}

type ServiceStatus struct {
	Name        string         `json:"name"`
	Namespace   string         `json:"namespace"`
	Type        string         `json:"type"`
	ClusterIP   string         `json:"clusterIP"`
	Endpoints   []EndpointInfo `json:"endpoints"`
	Ports       []ServicePort  `json:"ports"`
	Uptime      string         `json:"uptime"`
	LastFailure string         `json:"lastFailure,omitempty"`
	Health      string         `json:"health"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Protocol   string `json:"protocol"`
}
