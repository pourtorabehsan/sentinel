package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/pourtorabehsan/sentinel/internal/domain"
	"github.com/pourtorabehsan/sentinel/internal/storage"
)

type Handler struct {
	storage storage.Storage
}

func NewHandler(storage storage.Storage) *Handler {
	return &Handler{
		storage: storage,
	}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	// Cluster endpoints
	r.HandleFunc("/api/clusters", h.CreateCluster).Methods("POST")
	r.HandleFunc("/api/clusters", h.ListClusters).Methods("GET")
	r.HandleFunc("/api/clusters/{id}", h.GetCluster).Methods("GET")
	r.HandleFunc("/api/clusters/{id}", h.DeleteCluster).Methods("DELETE")

	// Instance endpoints
	r.HandleFunc("/api/clusters/{id}/instances", h.AddInstance).Methods("POST")
	r.HandleFunc("/api/clusters/{id}/instances", h.ListInstances).Methods("GET")
	r.HandleFunc("/api/instances/{instanceId}", h.GetInstance).Methods("GET")
	r.HandleFunc("/api/instances/{instanceId}", h.DeleteInstance).Methods("DELETE")
}

func (h *Handler) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	cluster := domain.Cluster{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
	}

	if err := h.storage.AddCluster(r.Context(), cluster); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, ClusterResponse{Cluster: cluster})
}

func (h *Handler) AddInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterID := vars["id"]

	var req AddInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	instance := domain.Instance{
		ID:        uuid.New().String(),
		Host:      req.Host,
		Port:      req.Port,
		Role:      req.Role,
		ClusterID: clusterID,
	}

	creds := domain.Credentials{
		Username: req.Username,
		Password: req.Password,
	}

	if err := h.storage.AddInstance(r.Context(), instance, creds); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	health, _ := h.storage.GetInstanceHealth(r.Context(), instance.ID)
	respondWithJSON(w, http.StatusCreated, InstanceResponse{
		Instance: instance,
		Health:   *health,
	})
}

func (h *Handler) ListClusters(w http.ResponseWriter, r *http.Request) {
	clusters, err := h.storage.ListClusters(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ClusterResponse, 0, len(clusters))
	for _, cluster := range clusters {
		instances, _ := h.storage.ListInstances(r.Context(), cluster.ID)
		response = append(response, ClusterResponse{
			Cluster:   cluster,
			Instances: instances,
		})
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *Handler) GetCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterID := vars["id"]

	cluster, err := h.storage.GetCluster(r.Context(), clusterID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	instances, _ := h.storage.ListInstances(r.Context(), clusterID)
	respondWithJSON(w, http.StatusOK, ClusterResponse{
		Cluster:   *cluster,
		Instances: instances,
	})
}

func (h *Handler) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterID := vars["id"]

	if err := h.storage.DeleteCluster(r.Context(), clusterID); err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Cluster deleted successfully"})
}

func (h *Handler) ListInstances(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterID := vars["id"]

	instances, err := h.storage.ListInstances(r.Context(), clusterID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get health status for each instance
	response := make([]InstanceResponse, 0, len(instances))
	for _, instance := range instances {
		health, err := h.storage.GetInstanceHealth(r.Context(), instance.ID)
		if err != nil {
			health = &domain.InstanceHealth{
				IsAlive:     false,
				LastChecked: time.Now(),
				Error:       "Health status not available",
			}
		}
		response = append(response, InstanceResponse{
			Instance: instance,
			Health:   *health,
		})
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *Handler) GetInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	instanceID := vars["instanceId"]

	instance, err := h.storage.GetInstance(r.Context(), instanceID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	health, err := h.storage.GetInstanceHealth(r.Context(), instanceID)
	if err != nil {
		health = &domain.InstanceHealth{
			IsAlive:     false,
			LastChecked: time.Now(),
			Error:       "Health status not available",
		}
	}

	respondWithJSON(w, http.StatusOK, InstanceResponse{
		Instance: *instance,
		Health:   *health,
	})
}

func (h *Handler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	instanceID := vars["instanceId"]

	if err := h.storage.DeleteInstance(r.Context(), instanceID); err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Instance deleted successfully"})
}

// Helper functions for HTTP responses
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{Error: message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
