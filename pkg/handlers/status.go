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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"strings"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type PodInfo struct {
	Total  int            `json:"total"`
	States map[string]int `json:"states"`
}

type OscarInfo struct {
	DeploymentName  string         `json:"deploymentName"`
	DeploymentReady bool           `json:"deploymentReady"`
	JobsCount       map[string]int `json:"jobsCount"` // "active", "succeeded", "failed"
	PodsInfo        PodInfo        `json:"podsInfo"`
}

type MinioBucketInfo struct {
	Name       string `json:"name"`
	PolicyType string `json:"policy_type"` // private, public, restricted
	PolicyJSON string `json:"policy_json"` // el JSON completo
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
func MakeStatusHandler(kubeClientset kubernetes.Interface, metricsClientset versioned.MetricsV1beta1Interface) gin.HandlerFunc {
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

		jobs, err := kubeClientset.BatchV1().Jobs("oscar-svc").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting OSCAR jobs: %v\n", err)
			os.Exit(1)
		}
		jobCounts := map[string]int{"active": 0, "succeeded": 0, "failed": 0}
		for _, job := range jobs.Items {
			jobCounts["active"] += int(job.Status.Active)
			jobCounts["succeeded"] += int(job.Status.Succeeded)
			jobCounts["failed"] += int(job.Status.Failed)
		}

		pods, err := kubeClientset.CoreV1().Pods("oscar-svc").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting OSCAR pods: %v\n", err)
			os.Exit(1)
		}
		podStates := map[string]int{}
		for _, pod := range pods.Items {
			podStates[string(pod.Status.Phase)]++
		}

		podInfo := PodInfo{
			Total:  len(pods.Items),
			States: podStates,
		}

		//MinIO info
		// Configura conexión MinIO
		//obtiene las variables del deplyment de OSCAR
		minioEndpoint := os.Getenv("MINIO_ENDPOINT")
		minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
		minioSecretKey := os.Getenv("MINIO_SECRET_KEY")

		//Parseamos la URL de minio_endpoint
		u, err := url.Parse(minioEndpoint)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing MINIO_ENDPOINT: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error configurando MinIO endpoint"})
			return
		}

		host := u.Host
		secure := u.Scheme == "https"
		// Crear cliente MinIO
		minioClient, err := minio.New(host, &minio.Options{
			Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
			Secure: secure,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creando cliente MinIO: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error conectando a MinIO"})
			return
		}

		// Listar buckets
		buckets, err := minioClient.ListBuckets(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listando buckets MinIO: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error listando buckets MinIO"})
			return
		}

		var bucketInfos []MinioBucketInfo

		for _, bucket := range buckets {
			policyStr := "unknown"
			policyJSON := ""

			policy, err := minioClient.GetBucketPolicy(context.Background(), bucket.Name)
			if err != nil {
				if responseErr, ok := err.(minio.ErrorResponse); ok && responseErr.Code == "NoSuchBucketPolicy" {
					policyStr = "private"
				} else {
					fmt.Fprintf(os.Stderr, "Error obteniendo política del bucket %s: %v\n", bucket.Name, err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error obteniendo política del bucket"})
					return
				}
			} else {
				policyJSON = policy

				if policy == "" {
					policyStr = "private"
					//Si la política se encuentra vacía tmb la asignamos como privada
				} else if strings.TrimSpace(policy) == `{"Version":"2012-10-17","Statement":[]}` {
					policyStr = "private"
				} else {
					var parsed map[string]interface{}
					if err := json.Unmarshal([]byte(policy), &parsed); err != nil {
						fmt.Fprintf(os.Stderr, "Error parseando política del bucket %s: %v\n", bucket.Name, err)
						policyStr = "unknown"
					} else {
						isPublic := false
						if stmts, ok := parsed["Statement"].([]interface{}); ok {
							for _, stmtIface := range stmts {
								if stmt, ok := stmtIface.(map[string]interface{}); ok {
									if effect, ok := stmt["Effect"].(string); ok && effect == "Allow" {
										principal := stmt["Principal"]
										switch p := principal.(type) {
										case string:
											if p == "*" {
												isPublic = true
											}
										case map[string]interface{}:
											if aws, ok := p["AWS"]; ok {
												switch v := aws.(type) {
												case string:
													if v == "*" {
														isPublic = true
													}
												case []interface{}:
													for _, entry := range v {
														if s, ok := entry.(string); ok && s == "*" {
															isPublic = true
														}
													}
												}
											}
										}
									}
								}
							}
						}
						if isPublic {
							policyStr = "public"
						} else {
							policyStr = "restricted"
						}
					}
				}
			}

			bucketInfos = append(bucketInfos, MinioBucketInfo{
				Name:       bucket.Name,
				PolicyType: policyStr,
				PolicyJSON: policyJSON,
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
				JobsCount:       jobCounts,
				PodsInfo:        podInfo,
			},
			MinIO: MinioInfo{
				Buckets: bucketInfos,
			},
		}

		c.JSON(http.StatusOK, clusterInfo)
	}
}
