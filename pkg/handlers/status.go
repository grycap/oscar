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
	Conditions       []NodeCondition `json:"conditions"`
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// MakeStatusHandler Status handler for kubernetes deployment.
func MakeStatusHandler(cfg *types.Config, kubeClientset kubernetes.Interface, metricsClientset versioned.MetricsV1beta1Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get  nodes list
		nodes, err := kubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting nodes list: %v\n", err)
			os.Exit(1)
		}

		// Get metrics nodes.
		nodeMetricsList, err := metricsClientset.NodeMetricses().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting metrics nodes: %v\n", err)
			os.Exit(1)
		}

		var nodeInfoList []NodeInfo
		var clusterInfo GeneralInfo

		var cpu_free_total int64 = 0
		var cpu_max_free int64 = 0
		var memory_free_total int64 = 0
		var memory_max_free int64 = 0
		var number_nodes int64 = 0

		//var for GPU info
		var hasGPU bool = false
		var totalGPUs int64 = 0

		// Parameters CPU and Memory.
		for id, node := range nodes.Items {
			//remove fronted node from json list
			if id > 0 {
				nodeName := node.Name
				cpu_alloc := node.Status.Allocatable.Cpu().MilliValue()
				cpu_usage := nodeMetricsList.Items[id].Usage["cpu"]
				cpu_usage_percent := (float64(cpu_usage.MilliValue()) / float64(cpu_alloc)) * 100

				memory_alloc := node.Status.Allocatable.Memory().Value()
				memory_usage := nodeMetricsList.Items[id].Usage["memory"]
				memory_usage_percent := (float64(memory_usage.Value()) / float64(memory_alloc)) * 100

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

				nodeInfo := NodeInfo{
					NodeName:         nodeName,
					CPUCapacity:      strconv.Itoa(int(cpu_alloc)),
					CPUUsage:         strconv.Itoa(int(cpu_usage.MilliValue())),
					CPUPercentage:    fmt.Sprintf("%.2f", cpu_usage_percent),
					MemoryCapacity:   strconv.Itoa(int(memory_alloc)),
					MemoryUsage:      strconv.Itoa(int(memory_usage.Value())),
					MemoryPercentage: fmt.Sprintf("%.2f", memory_usage_percent),
					Conditions:       conditions,
				}

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

				nodeInfoList = append(nodeInfoList, nodeInfo)
			}

		}

		// Obtener estado del deployment OSCAR pods y jobs
		deploymentsClient := kubeClientset.AppsV1().Deployments("oscar")
		deployment, err := deploymentsClient.Get(context.Background(), "oscar", metav1.GetOptions{})
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

		jobs, err := kubeClientset.BatchV1().Jobs("oscar-svc").List(context.Background(), metav1.ListOptions{})
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
		pods, err := kubeClientset.CoreV1().Pods("oscar-svc").List(context.Background(), metav1.ListOptions{})
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
		//creamos el adminClient
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

		clusterInfo = GeneralInfo{
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

		c.JSON(http.StatusOK, clusterInfo)
	}
}
