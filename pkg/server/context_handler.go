package server

import (
	"encoding/json"
	"github.com/x0ddf/tiny-status-page/pkg/utils"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
)

type contextResponse struct {
	Current  string   `json:"current"`
	Contexts []string `json:"contexts"`
}

type contextRequest struct {
	Context string `json:"context"`
}

func (s *Server) HandleContextList(w http.ResponseWriter, r *http.Request) {
	if utils.IsRunningInCluster() {
		http.Error(w, "Context switching is not available when running in-cluster", http.StatusBadRequest)
		return
	}

	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		http.Error(w, "Failed to load kubeconfig", http.StatusInternalServerError)
		return
	}

	contexts := make([]string, 0, len(config.Contexts))
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}

	response := contextResponse{
		Current:  config.CurrentContext,
		Contexts: contexts,
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	err = encoder.Encode(response)
	if err != nil {
		return
	}
}

func (s *Server) HandleContextSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if utils.IsRunningInCluster() {
		http.Error(w, "Context switching is not available when running in-cluster", http.StatusBadRequest)
		return
	}

	var req contextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	c, e := buildConfig(s.kubeconfig, req.Context)
	if e != nil {
		http.Error(w, e.Error(), http.StatusInternalServerError)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		log.Printf("Failed to create clientset: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	version, err := clientset.ServerVersion()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	log.Printf("connected to the cluster: %s | %v", req.Context, version)
	s.client = clientset
	s.UpdateWatcher(clientset)
	w.WriteHeader(http.StatusOK)
}
