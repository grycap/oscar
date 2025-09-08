/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"strings"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"

	minio "github.com/minio/minio-go/v7"
)

type PodSummary struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type PodInfo struct {
	Pods   []PodSummary   `json:"pods"`
	Total  int            `json:"total"`
	States map[string]int `json:"states"`
}

type OscarInfo struct {
	DeploymentName  string                 `json:"deploymentName"`
	DeploymentReady bool                   `json:"deploymentReady"`
	DeploymentInfo  map[string]interface{} `json:"deploymentInfo"`
	JobsCount       map[string]int         `json:"jobsCount"` // "active", "succeeded", "failed"
	PodsInfo        PodInfo                `json:"podsInfo"`
	OIDC            OIDCInfo               `json:"OIDC"`
}

type OIDCInfo struct {
	Enabled bool     `json:"enabled"`
	Issuers []string `json:"issuers"`
	Groups  []string `json:"groups"`
}

type MinioBucketInfo struct {
	Name         string   `json:"name"`
	PolicyType   string   `json:"policy_type"`
	PolicyJSON   string   `json:"policy_json"`
	Owner        string   `json:"owner,omitempty"`
	Members      []string `json:"members,omitempty"`
	CreationDate string   `json:"creation_date,omitempty"`
	Size         int64    `json:"size"`        // en bytes
	NumObjects   int      `json:"num_objects"` // cantidad de objetos
}

type MinioInfo struct {
	Buckets []MinioBucketInfo `json:"buckets"`
}

type GeneralInfo struct {
	NumberNodes     int64      `json:"numberNodes"`
	CPUFreeTotal    int64      `json:"cpuFreeTotal"`
	CPUMaxFree      int64      `json:"cpuMaxFree"`
	MemoryFreeTotal int64      `json:"memoryFreeTotal"`
	MemoryMaxFree   int64      `json:"memoryMaxFree"`
	HasGPU          bool       `json:"hasGPU"`
	GPUsTotal       int64      `json:"gpusTotal"`
	DetailsNodes    []NodeInfo `json:"detail"`
	OSCAR           OscarInfo  `json:"OSCAR"`
	MinIO           MinioInfo  `json:"MinIO"`
}

type NodeCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type NodeInfo struct {
	NodeName         string          `json:"nodeName"`
	CPUCapacity      string          `json:"cpuCapacity"`
	CPUUsage         string          `json:"cpuUsage"`
	CPUPercentage    string          `json:"cpuPercentage"`
	MemoryCapacity   string          `json:"memoryCapacity"`
	MemoryUsage      string          `json:"memoryUsage"`
	MemoryPercentage string          `json:"memoryPercentage"`
	IsInterLink      bool            `json:"isInterLink"`
	HasGPU           bool            `json:"hasGPU"`
	Conditions       []NodeCondition `json:"conditions"`
}

// Enhanced struct to store both display strings and int64 values
type NodeInfoWithAllocatable struct {
	NodeInfo          NodeInfo
	CPUAllocatable    int64
	MemoryAllocatable int64
	NodeName          string `json:"nodeName"`
	CPUCapacity       string `json:"cpuCapacity"`
	CPUUsage          string `json:"cpuUsage"`
	CPUPercentage     string `json:"cpuPercentage"`
	MemoryCapacity    string `json:"memoryCapacity"`
	MemoryUsage       string `json:"memoryUsage"`
	MemoryPercentage  string `json:"memoryPercentage"`
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// MakeStatusHandler Status handler for kubernetes deployment.
func MakeStatusHandler(cfg *types.Config, kubeClientset kubernetes.Interface, metricsClientset versioned.MetricsV1beta1Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get nodes list
		nodes, err := kubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get nodes list"})
			return
		}

		// Get metrics nodes.
		nodeMetricsList, err := metricsClientset.NodeMetricses().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting metrics nodes: %v\n", err)
			os.Exit(1)
		}

		// Use a map to store nodeInfo with int64 allocatable values
		nodeInfoMap := make(map[string]*NodeInfoWithAllocatable)
		var cpu_free_total int64 = 0
		var cpu_max_free int64 = 0
		var memory_free_total int64 = 0
		var memory_max_free int64 = 0

		// First pass: Create nodeInfo entries for all nodes (except control plane)
		for _, node := range nodes.Items {
			// Skip control plane nodes by checking their roles
			if isControlPlaneNode(node) {
				continue
			}
			//var for GPU info
			var hasGPU bool = false
			var totalGPUs int64 = 0
			var isInterLink bool = false

			// Parameters CPU and Memory.

			nodeName := node.Name

			cpu_alloc := node.Status.Allocatable.Cpu().MilliValue()
			memory_alloc := node.Status.Allocatable.Memory().Value()

			// Check if node is interLink (look for specific labels or annotations)
			isInterLink = checkIfInterLinkNode(node)

			// Check if node has GPU (look for nvidia.com/gpu or amd.com/gpu resources)
			hasGPU = checkIfNodeHasGPU(node)

			//
			if gpuQty, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
				hasGPU = true
				gpuVal, _ := gpuQty.AsInt64()
				totalGPUs += gpuVal
			}

			// Obtener condiciones del nodo
			var conditions []NodeCondition
			for _, cond := range node.Status.Conditions {
				conditions = append(conditions, NodeCondition{
					Type:   string(cond.Type),
					Status: string(cond.Status),
				})
			}
			nodeInfoMap[nodeName] = &NodeInfoWithAllocatable{
				NodeInfo: NodeInfo{
					NodeName:         nodeName,
					CPUCapacity:      strconv.Itoa(int(cpu_alloc)),
					CPUUsage:         "0", // Default to 0
					CPUPercentage:    "0.00",
					MemoryCapacity:   strconv.Itoa(int(memory_alloc)),
					IsInterLink:      isInterLink,
					HasGPU:           hasGPU,
					MemoryUsage:      "0", // Default to 0
					MemoryPercentage: "0.00",
					Conditions:       conditions,
				},
				CPUAllocatable:    cpu_alloc,
				MemoryAllocatable: memory_alloc,
			}
		}

		// Second pass: Going through nodeMetricsList, populating the rest of fields
		number_nodes := int64(0)
		var hasGPU bool = false
		var totalGPUs int64 = 0
		var nodeInfoList []NodeInfo
		for _, metrics := range nodeMetricsList.Items {
			nodeName := metrics.Name
			if nodeInfo, exists := nodeInfoMap[nodeName]; exists {
				// Use the stored int64 values directly (no string parsing!)
				cpu_alloc := nodeInfo.CPUAllocatable
				memory_alloc := nodeInfo.MemoryAllocatable

				cpu_usage := metrics.Usage["cpu"]
				memory_usage := metrics.Usage["memory"]
				cpu_usage_percent := (float64(cpu_usage.MilliValue()) / float64(cpu_alloc)) * 100
				memory_usage_percent := (float64(memory_usage.Value()) / float64(memory_alloc)) * 100

				// Update the nodeInfo fields directly by name
				nodeInfo.NodeInfo.CPUUsage = strconv.Itoa(int(cpu_usage.MilliValue()))
				nodeInfo.NodeInfo.CPUPercentage = fmt.Sprintf("%.2f", cpu_usage_percent)
				nodeInfo.NodeInfo.MemoryUsage = strconv.Itoa(int(memory_usage.Value()))
				nodeInfo.NodeInfo.MemoryPercentage = fmt.Sprintf("%.2f", memory_usage_percent)

				// Calculate free resources for cluster totals

				number_nodes++
				cpu_node_free := cpu_alloc - cpu_usage.MilliValue()
				cpu_free_total += cpu_node_free

				if cpu_max_free < cpu_node_free {
					cpu_max_free = cpu_node_free
				}

				memory_node_free := memory_alloc - memory_usage.Value()
				memory_free_total += memory_node_free
				if memory_max_free < memory_node_free {
					memory_max_free = memory_node_free
				}

				nodeInfoList = append(nodeInfoList, nodeInfo.NodeInfo)
			}

		}

		// Obtener estado del deployment OSCAR pods y jobs
		deploymentsClient := kubeClientset.AppsV1().Deployments(cfg.Namespace)
		deployment, err := deploymentsClient.Get(context.Background(), cfg.Namespace, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error getting OSCAR deployment: %v", err)})
			return
		}
		deploymentReady := deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
		oscarDeployment := deployment.Name

		//info sobre el deployment
		deploymentInfo := map[string]interface{}{
			"replicas":            deployment.Spec.Replicas,
			"readyReplicas":       deployment.Status.ReadyReplicas,
			"availableReplicas":   deployment.Status.AvailableReplicas,
			"unavailableReplicas": deployment.Status.UnavailableReplicas,
			"strategy":            deployment.Spec.Strategy.Type,
			"labels":              deployment.Labels,
			"creationTimestamp":   deployment.CreationTimestamp,
		}

		jobs, err := kubeClientset.BatchV1().Jobs(cfg.ServicesNamespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting OSCAR jobs: %v\n", err)
			os.Exit(1)
		}

		//Jobs info
		jobCounts := map[string]int{"active": 0, "succeeded": 0, "failed": 0}
		for _, job := range jobs.Items {
			jobCounts["active"] += int(job.Status.Active)
			jobCounts["succeeded"] += int(job.Status.Succeeded)
			jobCounts["failed"] += int(job.Status.Failed)
		}

		//Pods info
		pods, err := kubeClientset.CoreV1().Pods(cfg.ServicesNamespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting OSCAR pods: %v\n", err)
			os.Exit(1)
		}
		// Inicializar todos los posibles estados con 0
		podStates := map[string]int{
			"Pending":   0,
			"Running":   0,
			"Succeeded": 0,
			"Failed":    0,
			"Unknown":   0,
		}

		podSummaries := []PodSummary{}

		for _, pod := range pods.Items {
			state := string(pod.Status.Phase)
			podStates[state]++

			podSummaries = append(podSummaries, PodSummary{
				Name:  pod.Name,
				State: state,
			})
		}

		podInfo := PodInfo{
			Pods:   podSummaries,
			Total:  len(pods.Items),
			States: podStates,
		}

		//MinIO info
		adminClient, err := utils.MakeMinIOAdminClient(cfg)
		minioClient := adminClient.GetSimpleClient()
		if err != nil {
			log.Printf("Error creating MinIO admin client: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating MinIO client"})
			return
		}
		//cliente s3 para poder listar todos los buckets del cluster
		s3Client := cfg.MinIOProvider.GetS3Client()
		//listado de todos los buckets
		bucketList, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing buckets"})
			return
		}

		var bucketInfos []MinioBucketInfo

		for _, b := range bucketList.Buckets {
			bucketName := *b.Name
			visibility := adminClient.GetCurrentResourceVisibility(utils.MinIOBucket{BucketPath: bucketName})
			if err != nil {
				log.Printf("Error obtaining bucket visibility %s: %v", bucketName, err)
				visibility = "unknown"
			}
			metadata, metaErr := adminClient.GetTaggedMetadata(bucketName)
			if metaErr != nil {
				log.Printf("Error obtaining metadata from the bucket %s: %v", bucketName, metaErr)
				metadata = map[string]string{}
			}
			owner := metadata["owner"]
			var members []string

			//para los restricted sus miembros
			if visibility == utils.RESTRICTED {
				m, memberErr := adminClient.GetBucketMembers(bucketName)
				if memberErr != nil {
					log.Printf("Error obtaining bucket members %s: %v", bucketName, memberErr)
				} else {
					members = m
				}
			}

			var creationDate string
			if b.CreationDate != nil {
				creationDate = b.CreationDate.Format("2006-01-02T15:04:05Z07:00") // formato RFC3339
			}

			var totalSize int64 = 0
			var objectCount int = 0

			objectCh := minioClient.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{
				Recursive: true,
			})

			for obj := range objectCh {
				if obj.Err != nil {
					log.Printf("Error listing object in bucket %s: %v", bucketName, obj.Err)
					continue
				}
				totalSize += obj.Size
				objectCount++
			}

			bucketInfos = append(bucketInfos, MinioBucketInfo{
				Name:         bucketName,
				PolicyType:   visibility,
				Owner:        owner,
				Members:      members,
				CreationDate: creationDate,
				Size:         totalSize,
				NumObjects:   objectCount,
			})
		}

		clusterInfo := GeneralInfo{
			NumberNodes:     number_nodes,
			CPUFreeTotal:    cpu_free_total,
			CPUMaxFree:      cpu_max_free,
			MemoryFreeTotal: memory_free_total,
			MemoryMaxFree:   memory_max_free,
			DetailsNodes:    nodeInfoList,
			HasGPU:          hasGPU,
			GPUsTotal:       totalGPUs,
			OSCAR: OscarInfo{
				DeploymentName:  oscarDeployment,
				DeploymentReady: deploymentReady,
				DeploymentInfo:  deploymentInfo,
				JobsCount:       jobCounts,
				PodsInfo:        podInfo,
				OIDC: OIDCInfo{
					Enabled: cfg.OIDCEnable,
					Issuers: cfg.OIDCValidIssuers,
					Groups:  cfg.OIDCGroups,
				},
			},
			MinIO: MinioInfo{
				Buckets: bucketInfos,
			},
		}

		// Convert map to slice for JSON response
		for _, nodeInfo := range nodeInfoMap {
			nodeInfoList = append(nodeInfoList, nodeInfo.NodeInfo)
		}

		/*// Create cluster status structure (only once, outside loops)
		clusterInfo := GeneralInfo{
			NumberNodes:     int64(len(nodeInfoMap)), // Use map size instead
			CPUFreeTotal:    cpu_free_total,
			CPUMaxFree:      cpu_max_free,
			MemoryFreeTotal: memory_free_total,
			MemoryMaxFree:   memory_max_free,
			DetailsNodes:    nodeInfoList,
		}*/

		// Encode list of NodeInfo structures in json format.
		c.JSON(http.StatusOK, clusterInfo)
	}
}

// Helper function to check if a node is an interLink node
func checkIfInterLinkNode(node v1.Node) bool {
	// Check for the specific interLink label
	if nodeType, exists := node.Labels["virtual-node.interlink/type"]; exists && nodeType == "virtual-kubelet" {
		return true
	}
	return false
}

// Helper function to check if a node has GPU
func checkIfNodeHasGPU(node v1.Node) bool {
	// Check for NVIDIA GPU resources in allocatable
	if gpu, exists := node.Status.Allocatable["nvidia.com/gpu"]; exists && !gpu.IsZero() {
		return true
	}
	return false
}

// Helper function to check if a node is a control plane node
func isControlPlaneNode(node v1.Node) bool {
	// Check for control-plane role label (Kubernetes 1.20+)
	if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
		return true
	}

	// Check for master role label (older Kubernetes versions)
	if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
		return true
	}

	return false
}
