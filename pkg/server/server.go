package server

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/x0ddf/kube-status-page/pkg/types"
)

type Server struct {
	tmpl     *template.Template
	upgrader websocket.Upgrader
	mu       sync.RWMutex
	services map[string]*types.ServiceStatus
}

// Add this struct to handle WebSocket messages
type WSMessage struct {
	Type    string                  `json:"type"`
	Payload []*types.NamespaceGroup `json:"payload"`
}

func NewServer() (*Server, error) {
	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		tmpl: tmpl,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // For development, you might want to restrict this in production
			},
		},
		services: make(map[string]*types.ServiceStatus),
	}, nil
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
	json.NewEncoder(w).Encode(groups)
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
