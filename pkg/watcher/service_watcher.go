package watcher

import (
	"context"
	"fmt"
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

	// Get pods for this service
	pods, err := w.getPodsForService(service)
	if err != nil {
		log.Printf("Error getting pods for service %s/%s: %v", service.Namespace, service.Name, err)
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
		Uptime:    calculateUptimeFromPods(pods),
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
