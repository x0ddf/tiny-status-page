package server

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/x0ddf/tiny-status-page/pkg/utils"
	"github.com/x0ddf/tiny-status-page/pkg/watcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"

	"github.com/gorilla/websocket"
	"github.com/x0ddf/tiny-status-page/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func buildConfig(kubeconfigPath, ctx string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			ClusterInfo:    clientcmdapi.Cluster{Server: ""},
			CurrentContext: ctx,
		}).ClientConfig()
}

func getKubeConfig() (string, *rest.Config, error) {
	// Try in-cluster config first
	if utils.IsRunningInCluster() {
		c, err := rest.InClusterConfig()
		return "", c, err
	}
	// Fallback to local kubeconfig
	configPath := configPath()
	c, e := clientcmd.BuildConfigFromFlags("", configPath)
	// use the current context in kubeconfig
	return configPath, c, e
}

func configPath() string {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	return *kubeconfig
}

type Server struct {
	tmpl           *template.Template
	upgrader       websocket.Upgrader
	mu             sync.RWMutex
	services       map[string]*types.ServiceStatus
	serviceWatcher *watcher.ServiceWatcher
	InCluster      bool
	kubeconfig     string
	client         *kubernetes.Clientset
}

// WSMessage this struct handles WebSocket messages
type WSMessage struct {
	Type    string                  `json:"type"`
	Payload []*types.NamespaceGroup `json:"payload"`
}

func (s *Server) init() {

}

func NewServer() (*Server, error) {
	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		return nil, err
	}
	// Create kubernetes client
	c, config, err := getKubeConfig()
	if err != nil {
		log.Fatalf("Failed to get cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Initialize service watcher
	srv := &Server{
		tmpl:      tmpl,
		InCluster: utils.IsRunningInCluster(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // For development, you might want to restrict this in production
			},
		},
		services:   make(map[string]*types.ServiceStatus),
		client:     clientset,
		kubeconfig: c,
	}
	srv.serviceWatcher = watcher.NewServiceWatcher(srv.client)
	go srv.serviceWatcher.Run(srv.UpdateService)

	return srv, nil
}

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.tmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleServices(w http.ResponseWriter, r *http.Request) {
	groups := s.groupServicesByNamespace()
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	err := encoder.Encode(groups)
	if err != nil {
		return
	}
}

func (s *Server) groupServicesByNamespace() []*types.NamespaceGroup {
	// Create a map to group services
	groups := make(map[string][]*types.ServiceStatus)

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Group services by namespace
	for _, svc := range s.services {
		groups[svc.Namespace] = append(groups[svc.Namespace], svc)
	}

	// Convert map to sorted slice
	result := make([]*types.NamespaceGroup, 0, len(groups))
	for ns, services := range groups {
		// Sort services by name
		sort.Slice(services, func(i, j int) bool {
			return services[i].Name < services[j].Name
		})

		result = append(result, &types.NamespaceGroup{
			Namespace: ns,
			Services:  services,
		})
	}

	// Sort namespaces
	sort.Slice(result, func(i, j int) bool {
		return result[i].Namespace < result[j].Namespace
	})

	return result
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Create done channel for cleanup
	done := make(chan struct{})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(done)
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			groups := s.groupServicesByNamespace()
			if err := conn.WriteJSON(groups); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket write failed: %v", err)
				}
				return
			}
		}
	}
}

// UpdateService updates the service status and notifies all connected clients
func (s *Server) UpdateService(status *types.ServiceStatus) {
	s.mu.Lock()
	s.services[status.Name] = status
	s.mu.Unlock()
}

func GetAvailableContexts() []string {
	// Load kubeconfig
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		log.Printf("Failed to load kubeconfig: %v", err)
		return nil
	}

	// Get all context names
	contexts := make([]string, 0, len(config.Contexts))
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}

	return contexts
}

func GetCurrentContext() string {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		log.Printf("Failed to get current context: %v", err)
		return ""
	}
	return config.CurrentContext
}

func (s *Server) UpdateWatcher(client *kubernetes.Clientset) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing watcher if any
	if s.serviceWatcher != nil {
		s.serviceWatcher.Stop()
	}

	// Create new watcher
	s.serviceWatcher = watcher.NewServiceWatcher(client)
	go s.serviceWatcher.Run(s.UpdateService)
}
