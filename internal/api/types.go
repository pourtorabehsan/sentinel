package api

import "github.com/pourtorabehsan/sentinel/internal/domain"

// Request types
type CreateClusterRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AddInstanceRequest struct {
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Role     domain.InstanceRole `json:"role"`
	Username string            `json:"username"`
	Password string            `json:"password"`
}

// Response types
type ErrorResponse struct {
	Error string `json:"error"`
}

type ClusterResponse struct {
	Cluster  domain.Cluster    `json:"cluster"`
	Instances []domain.Instance `json:"instances,omitempty"`
}

type InstanceResponse struct {
	Instance domain.Instance        `json:"instance"`
	Health   domain.InstanceHealth `json:"health"`
}
