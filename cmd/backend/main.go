package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/x0ddf/kube-status-page/pkg/server"
	"github.com/x0ddf/kube-status-page/pkg/watcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const DefaultPort = "8080"
const PortVar = "PORT"

func isRunningInCluster() bool {
	// Check if the service account token file exists
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return true
	}
	return false
}

func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	if isRunningInCluster() {
		return rest.InClusterConfig()
	}

	// Fallback to local kubeconfig
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	return clientcmd.BuildConfigFromFlags("", *kubeconfig)
}

func main() {
	// Create kubernetes client
	config, err := getKubeConfig()
	if err != nil {
		log.Fatalf("Failed to get cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	var port string
	if port = os.Getenv(PortVar); port == "" {
		port = DefaultPort
	}

	// Initialize server
	srv, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Initialize service watcher
	serviceWatcher := watcher.NewServiceWatcher(clientset)
	go serviceWatcher.Run(srv.UpdateService)

	// Setup HTTP handlers
	http.HandleFunc("/", srv.HandleIndex)
	http.HandleFunc("/api/services", srv.HandleServices)
	http.HandleFunc("/ws", srv.HandleWebSocket)

	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
