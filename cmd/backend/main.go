package main

import (
	"flag"
	"log"
	"net/http"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/x0ddf/kube-status-page/pkg/server"
	"github.com/x0ddf/kube-status-page/pkg/watcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func local() (*rest.Config, error) {
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
func cluster() (*rest.Config, error) {
	return rest.InClusterConfig()
}
func main() {
	// Create kubernetes client
	config, err := local()
	if err != nil {
		log.Fatalf("Failed to get cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Initialize server
	srv, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Initialize service watcher
	watcher := watcher.NewServiceWatcher(clientset)
	go watcher.Run(srv.UpdateService)

	// Setup HTTP handlers
	http.HandleFunc("/", srv.HandleIndex)
	http.HandleFunc("/api/services", srv.HandleServices)
	http.HandleFunc("/ws", srv.HandleWebSocket)

	log.Printf("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
