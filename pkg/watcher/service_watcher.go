package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/x0ddf/tiny-status-page/pkg/types"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
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

	// Get pods and endpoints
	pods, err := w.getPodsForService(service)
	if err != nil {
		log.Printf("Error getting pods for service %s/%s: %v", service.Namespace, service.Name, err)
	}

	endpoints := w.getEndpoints(service)
	ports := w.convertPorts(service.Spec.Ports)

	// Calculate health status
	health := "Unhealthy"
	if len(endpoints) > 0 && hasRunningPod(pods) {
		health = "Healthy"
	}

	status := &types.ServiceStatus{
		Name:      service.Name,
		Namespace: service.Namespace,
		Type:      string(service.Spec.Type),
		ClusterIP: service.Spec.ClusterIP,
		Endpoints: endpoints,
		Ports:     ports,
		CreatedAt: service.CreationTimestamp.Time,
		Uptime:    calculateUptimeFromPods(pods),
		Health:    health,
	}

	w.services[service.Name] = status
	w.updateFunc(status)
}

func (w *ServiceWatcher) getPodsForService(svc *corev1.Service) ([]corev1.Pod, error) {
	if svc.Spec.Selector == nil {
		return nil, nil
	}

	labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{
		MatchLabels: svc.Spec.Selector,
	})

	pods, err := w.client.CoreV1().Pods(svc.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func calculateUptimeFromPods(pods []corev1.Pod) string {
	if len(pods) == 0 {
		return "N/A"
	}

	var oldestPodTime *time.Time
	for _, pod := range pods {
		// Only consider running pods
		if pod.Status.Phase == corev1.PodRunning {
			podStartTime := pod.Status.StartTime
			if podStartTime != nil {
				if oldestPodTime == nil || podStartTime.Time.Before(*oldestPodTime) {
					oldestPodTime = &podStartTime.Time
				}
			}
		}
	}

	if oldestPodTime == nil {
		return "N/A"
	}

	duration := time.Since(*oldestPodTime)
	return formatDuration(duration)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func hasRunningPod(pods []corev1.Pod) bool {
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning {
			return true
		}
	}
	return false
}

func (w *ServiceWatcher) getEndpoints(service *corev1.Service) []types.EndpointInfo {
	endpoints := []types.EndpointInfo{}

	// List EndpointSlices for this service
	slices, err := w.client.DiscoveryV1().EndpointSlices(service.Namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("kubernetes.io/service-name=%s", service.Name),
		},
	)
	if err != nil {
		log.Printf("Error getting EndpointSlices for service %s/%s: %v",
			service.Namespace, service.Name, err)
		return endpoints
	}

	// Collect endpoints from all slices
	for _, slice := range slices.Items {
		if slice.AddressType != discoveryv1.AddressTypeIPv4 {
			continue
		}

		for _, endpoint := range slice.Endpoints {
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				// Get pod name from endpoint
				podName := ""
				if endpoint.TargetRef != nil && endpoint.TargetRef.Kind == "Pod" {
					podName = endpoint.TargetRef.Name
				} else if endpoint.Hostname != nil {
					podName = *endpoint.Hostname
				}

				// Add each address with its pod name
				for _, addr := range endpoint.Addresses {
					endpoints = append(endpoints, types.EndpointInfo{
						PodName: podName,
						IP:      addr,
					})
				}
			}
		}
	}

	return endpoints
}

func (w *ServiceWatcher) convertPorts(servicePorts []corev1.ServicePort) []types.ServicePort {
	ports := make([]types.ServicePort, len(servicePorts))
	for i, p := range servicePorts {
		ports[i] = types.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: p.TargetPort.IntVal,
			Protocol:   string(p.Protocol),
		}
	}
	return ports
}
