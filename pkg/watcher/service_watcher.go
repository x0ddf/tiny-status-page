package watcher

import (
	"context"
	"log"
	"time"

	"github.com/x0ddf/kube-status-page/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	//"k8s.io/client-go/tools/watch"
)

type ServiceWatcher struct {
	client     *kubernetes.Clientset
	services   map[string]*types.ServiceStatus
	updateFunc func(*types.ServiceStatus)
}

func NewServiceWatcher(client *kubernetes.Clientset) *ServiceWatcher {
	return &ServiceWatcher{
		client:   client,
		services: make(map[string]*types.ServiceStatus),
	}
}

func (w *ServiceWatcher) Run(updateFunc func(*types.ServiceStatus)) {
	w.updateFunc = updateFunc

	watcher, err := w.client.CoreV1().Services("").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to watch services: %v", err)
	}

	for event := range watcher.ResultChan() {
		w.handleServiceEvent(event)
	}
}

func (w *ServiceWatcher) handleServiceEvent(event watch.Event) {
	service, ok := event.Object.(*corev1.Service)
	if !ok {
		return
	}

	// Get endpoints
	endpoints := []string{}
	if eps, err := w.client.CoreV1().Endpoints(service.Namespace).Get(context.Background(), service.Name, metav1.GetOptions{}); err == nil {
		for _, subset := range eps.Subsets {
			for _, addr := range subset.Addresses {
				endpoints = append(endpoints, addr.IP)
			}
		}
	}

	// Convert ports
	ports := make([]types.ServicePort, len(service.Spec.Ports))
	for i, p := range service.Spec.Ports {
		ports[i] = types.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: p.TargetPort.IntVal,
			Protocol:   string(p.Protocol),
		}
	}

	status := &types.ServiceStatus{
		Name:      service.Name,
		Namespace: service.Namespace,
		Type:      string(service.Spec.Type),
		ClusterIP: service.Spec.ClusterIP,
		Endpoints: endpoints,
		Ports:     ports,
		CreatedAt: service.CreationTimestamp.Time,
		Uptime:    time.Since(service.CreationTimestamp.Time).Round(time.Second).String(),
	}

	w.services[service.Name] = status
	w.updateFunc(status)
}
