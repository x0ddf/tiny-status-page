package main

import (
	"flag"
	"github.com/x0ddf/tiny-status-page/pkg/utils"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/x0ddf/tiny-status-page/pkg/server"
	"k8s.io/client-go/rest"
)

const DefaultPort = "8080"
const PortVar = "PORT"

func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	if utils.IsRunningInCluster() {
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
	var port string
	if port = os.Getenv(PortVar); port == "" {
		port = DefaultPort
	}

	// Initialize server
	srv, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup HTTP handlers
	http.HandleFunc("/", srv.HandleIndex)
	http.HandleFunc("/ws", srv.HandleWebSocket)
	http.HandleFunc("/api/services", srv.HandleServices)
	http.HandleFunc("/api/contexts", srv.HandleContextList)
	http.HandleFunc("/api/contexts/switch", srv.HandleContextSwitch)

	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
