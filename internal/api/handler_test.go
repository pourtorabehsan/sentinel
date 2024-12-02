package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/pourtorabehsan/sentinel/internal/domain"
	"github.com/pourtorabehsan/sentinel/internal/storage/memory"
)

func setupTestHandler() (*Handler, *memory.MemoryStorage) {
	storage := memory.NewMemoryStorage()
	handler := NewHandler(storage)
	return handler, storage
}

func TestHandler_CreateCluster(t *testing.T) {
	handler, _ := setupTestHandler()

	tests := []struct {
		name       string
		payload    CreateClusterRequest
		wantStatus int
	}{
		{
			name: "valid cluster",
			payload: CreateClusterRequest{
				Name:        "test-cluster",
				Description: "Test cluster",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "empty name",
			payload: CreateClusterRequest{
				Name:        "",
				Description: "Test cluster",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/clusters", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.CreateCluster(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("CreateCluster() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusCreated {
				var response ClusterResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				if response.Cluster.Name != tt.payload.Name {
					t.Errorf("Got cluster name %v, want %v", response.Cluster.Name, tt.payload.Name)
				}
			}
		})
	}
}

func TestHandler_AddInstance(t *testing.T) {
	handler, storage := setupTestHandler()
	ctx := context.Background()

	// Create a test cluster first
	cluster := domain.Cluster{
		ID:          "test-cluster",
		Name:        "Test Cluster",
		Description: "Test cluster for instance operations",
		CreatedAt:   time.Now(),
	}
	if err := storage.AddCluster(ctx, cluster); err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}

	tests := []struct {
		name       string
		clusterID  string
		payload    AddInstanceRequest
		wantStatus int
	}{
		{
			name:      "valid instance",
			clusterID: cluster.ID,
			payload: AddInstanceRequest{
				Host:     "localhost",
				Port:     3306,
				Role:     domain.RolePrimary,
				Username: "root",
				Password: "password",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:      "invalid cluster",
			clusterID: "non-existent",
			payload: AddInstanceRequest{
				Host:     "localhost",
				Port:     3306,
				Role:     domain.RolePrimary,
				Username: "root",
				Password: "password",
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/clusters/"+tt.clusterID+"/instances", bytes.NewBuffer(body))
			req = mux.SetURLVars(req, map[string]string{"id": tt.clusterID})
			w := httptest.NewRecorder()

			handler.AddInstance(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("AddInstance() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusCreated {
				var response InstanceResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				if response.Instance.Host != tt.payload.Host {
					t.Errorf("Got instance host %v, want %v", response.Instance.Host, tt.payload.Host)
				}
			}
		})
	}
}

func TestHandler_ListClusters(t *testing.T) {
	handler, storage := setupTestHandler()
	ctx := context.Background()

	// Create some test clusters
	clusters := []domain.Cluster{
		{
			ID:          "cluster-1",
			Name:        "Cluster 1",
			Description: "Test cluster 1",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "cluster-2",
			Name:        "Cluster 2",
			Description: "Test cluster 2",
			CreatedAt:   time.Now(),
		},
	}

	for _, cluster := range clusters {
		if err := storage.AddCluster(ctx, cluster); err != nil {
			t.Fatalf("Failed to create test cluster: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/api/clusters", nil)
	w := httptest.NewRecorder()

	handler.ListClusters(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListClusters() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response []ClusterResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(response) != len(clusters) {
		t.Errorf("Got %d clusters, want %d", len(response), len(clusters))
	}
}

func TestHandler_DeleteInstance(t *testing.T) {
	handler, storage := setupTestHandler()
	ctx := context.Background()

	// Create a test cluster and instance
	cluster := domain.Cluster{
		ID:          "test-cluster",
		Name:        "Test Cluster",
		Description: "Test cluster",
		CreatedAt:   time.Now(),
	}
	if err := storage.AddCluster(ctx, cluster); err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}

	instance := domain.Instance{
		ID:        "test-instance",
		Host:      "localhost",
		Port:      3306,
		Role:      domain.RolePrimary,
		ClusterID: cluster.ID,
	}
	creds := domain.Credentials{
		Username: "root",
		Password: "password",
	}
	if err := storage.AddInstance(ctx, instance, creds); err != nil {
		t.Fatalf("Failed to create test instance: %v", err)
	}

	tests := []struct {
		name       string
		instanceID string
		wantStatus int
	}{
		{
			name:       "existing instance",
			instanceID: instance.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent instance",
			instanceID: "non-existent",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/api/instances/"+tt.instanceID, nil)
			req = mux.SetURLVars(req, map[string]string{"instanceId": tt.instanceID})
			w := httptest.NewRecorder()

			handler.DeleteInstance(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("DeleteInstance() status = %v, want %v", w.Code, tt.wantStatus)
			}
		})
	}
}
