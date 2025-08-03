package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// K8sManager Kubernetes管理器
type K8sManager struct {
	server *UnlimitedControlServer
}

// NewK8sManager 创建Kubernetes管理器
func NewK8sManager(server *UnlimitedControlServer) *K8sManager {
	return &K8sManager{
		server: server,
	}
}

// ValidateKubeConfig 验证Kubernetes配置
func (km *K8sManager) ValidateKubeConfig(kubeconfig string) error {
	if kubeconfig == "" {
		return fmt.Errorf("kubeconfig is required")
	}
	
	// 这里可以添加更详细的kubeconfig验证逻辑
	// 例如解析YAML、验证证书等
	
	return nil
}

// GenerateOperatorManifest 生成Operator部署清单
func (km *K8sManager) GenerateOperatorManifest(config K8sOperatorConfig) (string, error) {
	manifest := fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: %s
  namespace: %s
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %s
rules:
- apiGroups: [""]
  resources: ["pods", "services", "nodes"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %s
subjects:
- kind: ServiceAccount
  name: %s
  namespace: %s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tailscale-operator
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tailscale-operator
  template:
    metadata:
      labels:
        app: tailscale-operator
    spec:
      serviceAccountName: %s
      containers:
      - name: operator
        image: %s
        env:
        - name: TAILSCALE_AUTHKEY
          valueFrom:
            secretKeyRef:
              name: tailscale-auth
              key: authkey
        - name: LOG_LEVEL
          value: %s
        - name: SYNC_INTERVAL
          value: "%d"
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 128Mi
---
apiVersion: v1
kind: Secret
metadata:
  name: tailscale-auth
  namespace: %s
type: Opaque
data:
  authkey: # Base64 encoded auth key
`, 
		config.Namespace, config.ServiceAccount, config.Namespace,
		config.ClusterRole, config.ClusterRole, config.ClusterRole,
		config.ServiceAccount, config.Namespace, config.Namespace,
		config.ServiceAccount, config.OperatorImage, config.LogLevel,
		config.SyncInterval, config.Namespace)

	return manifest, nil
}

// AllocateIPForPod 为Pod分配Tailscale IP
func (km *K8sManager) AllocateIPForPod(podID uint) (string, error) {
	var pod K8sPod
	if err := km.server.db.First(&pod, podID).Error; err != nil {
		return "", err
	}

	// 生成一个模拟的Tailscale IP（在实际实现中，这应该从IP池中分配）
	ip := fmt.Sprintf("100.64.%d.%d", (podID%254)+1, (podID%254)+1)
	
	pod.TailscaleIP = ip
	if err := km.server.db.Save(&pod).Error; err != nil {
		return "", err
	}

	return ip, nil
}

// ReleaseIPForPod 释放Pod的Tailscale IP
func (km *K8sManager) ReleaseIPForPod(podID uint) error {
	var pod K8sPod
	if err := km.server.db.First(&pod, podID).Error; err != nil {
		return err
	}

	pod.TailscaleIP = ""
	pod.DeviceID = 0
	return km.server.db.Save(&pod).Error
}

// SyncClusterResources 同步集群资源
func (km *K8sManager) SyncClusterResources(clusterID uint) error {
	var cluster K8sCluster
	if err := km.server.db.First(&cluster, clusterID).Error; err != nil {
		return err
	}

	// 在实际实现中，这里会连接到Kubernetes API服务器
	// 获取Pods、Services、Nodes等资源信息并同步到数据库
	
	// 模拟同步过程
	cluster.LastSync = time.Now()
	cluster.Status = "connected"
	
	return km.server.db.Save(&cluster).Error
}

// ===== 集群管理处理函数 =====

// handleListK8sClusters 获取Kubernetes集群列表
func (s *UnlimitedControlServer) handleListK8sClusters(c *gin.Context) {
	var clusters []K8sCluster
	
	query := s.db.Model(&K8sCluster{})
	
	// 支持按状态过滤
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&clusters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch K8s clusters",
		})
		return
	}

	// 隐藏敏感信息
	for i := range clusters {
		clusters[i].KubeConfig = ""
		clusters[i].Token = ""
		clusters[i].CACert = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    clusters,
		"total":   len(clusters),
	})
}

// handleCreateK8sCluster 创建Kubernetes集群
func (s *UnlimitedControlServer) handleCreateK8sCluster(c *gin.Context) {
	var req K8sCluster
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查名称是否已存在
	var existing K8sCluster
	if err := s.db.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Cluster with this name already exists",
		})
		return
	}

	// 验证必填字段
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Cluster name is required",
		})
		return
	}

	if req.APIServer == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "API server is required",
		})
		return
	}

	// 验证KubeConfig
	km := NewK8sManager(s)
	if err := km.ValidateKubeConfig(req.KubeConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid kubeconfig: " + err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Namespace == "" {
		req.Namespace = "default"
	}
	if req.PodCIDR == "" {
		req.PodCIDR = "10.244.0.0/16"
	}
	if req.ServiceCIDR == "" {
		req.ServiceCIDR = "10.96.0.0/12"
	}
	req.Status = "disconnected"

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create K8s cluster",
		})
		return
	}

	// 隐藏敏感信息
	req.KubeConfig = ""
	req.Token = ""
	req.CACert = ""

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetK8sCluster 获取单个Kubernetes集群
func (s *UnlimitedControlServer) handleGetK8sCluster(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var cluster K8sCluster
	if err := s.db.First(&cluster, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s cluster not found",
		})
		return
	}

	// 隐藏敏感信息
	cluster.KubeConfig = ""
	cluster.Token = ""
	cluster.CACert = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cluster,
	})
}

// handleUpdateK8sCluster 更新Kubernetes集群
func (s *UnlimitedControlServer) handleUpdateK8sCluster(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var req K8sCluster
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var cluster K8sCluster
	if err := s.db.First(&cluster, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s cluster not found",
		})
		return
	}

	// 检查名称冲突
	if req.Name != cluster.Name {
		var existing K8sCluster
		if err := s.db.Where("name = ? AND id != ?", req.Name, cluster.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Cluster with this name already exists",
			})
			return
		}
	}

	// 更新字段
	cluster.Name = req.Name
	cluster.Description = req.Description
	cluster.APIServer = req.APIServer
	cluster.Namespace = req.Namespace
	cluster.CNIEnabled = req.CNIEnabled
	cluster.CNIConfig = req.CNIConfig
	cluster.PodCIDR = req.PodCIDR
	cluster.ServiceCIDR = req.ServiceCIDR

	// 只有在提供了新的配置时才更新敏感字段
	if req.KubeConfig != "" {
		cluster.KubeConfig = req.KubeConfig
	}
	if req.Token != "" {
		cluster.Token = req.Token
	}
	if req.CACert != "" {
		cluster.CACert = req.CACert
	}

	if err := s.db.Save(&cluster).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update K8s cluster",
		})
		return
	}

	// 隐藏敏感信息
	cluster.KubeConfig = ""
	cluster.Token = ""
	cluster.CACert = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cluster,
	})
}

// handleDeleteK8sCluster 删除Kubernetes集群
func (s *UnlimitedControlServer) handleDeleteK8sCluster(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var cluster K8sCluster
	if err := s.db.First(&cluster, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s cluster not found",
		})
		return
	}

	// 删除相关资源
	s.db.Where("cluster_id = ?", cluster.ID).Delete(&K8sPod{})
	s.db.Where("cluster_id = ?", cluster.ID).Delete(&K8sService{})
	s.db.Where("cluster_id = ?", cluster.ID).Delete(&K8sNode{})
	s.db.Where("cluster_id = ?", cluster.ID).Delete(&K8sNetworkPolicy{})
	s.db.Where("cluster_id = ?", cluster.ID).Delete(&K8sOperatorConfig{})

	// 删除集群
	if err := s.db.Delete(&cluster).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete K8s cluster",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "K8s cluster deleted successfully",
	})
}

// handleConnectK8sCluster 连接Kubernetes集群
func (s *UnlimitedControlServer) handleConnectK8sCluster(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var cluster K8sCluster
	if err := s.db.First(&cluster, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s cluster not found",
		})
		return
	}

	// 模拟连接过程
	km := NewK8sManager(s)
	if err := km.SyncClusterResources(cluster.ID); err != nil {
		cluster.Status = "error"
		s.db.Save(&cluster)

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to connect to K8s cluster: " + err.Error(),
		})
		return
	}

	cluster.Status = "connected"
	cluster.LastSync = time.Now()
	s.db.Save(&cluster)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "K8s cluster connected successfully",
		"data":    cluster,
	})
}

// handleDisconnectK8sCluster 断开Kubernetes集群连接
func (s *UnlimitedControlServer) handleDisconnectK8sCluster(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var cluster K8sCluster
	if err := s.db.First(&cluster, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s cluster not found",
		})
		return
	}

	cluster.Status = "disconnected"
	if err := s.db.Save(&cluster).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to disconnect K8s cluster",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "K8s cluster disconnected successfully",
		"data":    cluster,
	})
}

// handleSyncK8sCluster 同步Kubernetes集群
func (s *UnlimitedControlServer) handleSyncK8sCluster(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var cluster K8sCluster
	if err := s.db.First(&cluster, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s cluster not found",
		})
		return
	}

	km := NewK8sManager(s)
	if err := km.SyncClusterResources(cluster.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to sync K8s cluster: " + err.Error(),
		})
		return
	}

	// 重新加载集群信息
	s.db.First(&cluster, cluster.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "K8s cluster synced successfully",
		"data":    cluster,
	})
}

// ===== Pod管理处理函数 =====

// handleListK8sPods 获取Kubernetes Pod列表
func (s *UnlimitedControlServer) handleListK8sPods(c *gin.Context) {
	var pods []K8sPod

	query := s.db.Model(&K8sPod{}).Preload("Cluster").Preload("Device")

	// 支持按集群过滤
	if clusterID := c.Query("cluster_id"); clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}

	// 支持按命名空间过滤
	if namespace := c.Query("namespace"); namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}

	// 支持按状态过滤
	if phase := c.Query("phase"); phase != "" {
		query = query.Where("phase = ?", phase)
	}

	if err := query.Order("created_at DESC").Find(&pods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch K8s pods",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pods,
		"total":   len(pods),
	})
}

// handleGetK8sPod 获取单个Kubernetes Pod
func (s *UnlimitedControlServer) handleGetK8sPod(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid pod ID",
		})
		return
	}

	var pod K8sPod
	if err := s.db.Preload("Cluster").Preload("Device").First(&pod, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s pod not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pod,
	})
}

// handleAssignPodIP 为Pod分配Tailscale IP
func (s *UnlimitedControlServer) handleAssignPodIP(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid pod ID",
		})
		return
	}

	var pod K8sPod
	if err := s.db.First(&pod, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s pod not found",
		})
		return
	}

	if pod.TailscaleIP != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Pod already has a Tailscale IP assigned",
		})
		return
	}

	km := NewK8sManager(s)
	ip, err := km.AllocateIPForPod(pod.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to assign IP to pod: " + err.Error(),
		})
		return
	}

	// 重新加载Pod信息
	s.db.First(&pod, pod.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Tailscale IP assigned successfully",
		"data": gin.H{
			"pod_id":       pod.ID,
			"tailscale_ip": ip,
		},
	})
}

// handleReleasePodIP 释放Pod的Tailscale IP
func (s *UnlimitedControlServer) handleReleasePodIP(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid pod ID",
		})
		return
	}

	var pod K8sPod
	if err := s.db.First(&pod, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s pod not found",
		})
		return
	}

	if pod.TailscaleIP == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Pod does not have a Tailscale IP assigned",
		})
		return
	}

	km := NewK8sManager(s)
	if err := km.ReleaseIPForPod(pod.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to release IP from pod: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Tailscale IP released successfully",
	})
}

// handleGetPodsByCluster 获取集群的Pod列表
func (s *UnlimitedControlServer) handleGetPodsByCluster(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("cluster_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var pods []K8sPod
	if err := s.db.Where("cluster_id = ?", uint(clusterID)).Order("created_at DESC").Find(&pods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch cluster pods",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pods,
		"total":   len(pods),
	})
}

// ===== Service管理处理函数 =====

// handleListK8sServices 获取Kubernetes Service列表
func (s *UnlimitedControlServer) handleListK8sServices(c *gin.Context) {
	var services []K8sService

	query := s.db.Model(&K8sService{}).Preload("Cluster")

	// 支持按集群过滤
	if clusterID := c.Query("cluster_id"); clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}

	// 支持按命名空间过滤
	if namespace := c.Query("namespace"); namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}

	if err := query.Order("created_at DESC").Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch K8s services",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    services,
		"total":   len(services),
	})
}

// handleGetK8sService 获取单个Kubernetes Service
func (s *UnlimitedControlServer) handleGetK8sService(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid service ID",
		})
		return
	}

	var service K8sService
	if err := s.db.Preload("Cluster").First(&service, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s service not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    service,
	})
}

// handleExposeK8sService 暴露Kubernetes Service
func (s *UnlimitedControlServer) handleExposeK8sService(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid service ID",
		})
		return
	}

	var service K8sService
	if err := s.db.First(&service, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s service not found",
		})
		return
	}

	if service.Exposed {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Service is already exposed",
		})
		return
	}

	// 分配Tailscale IP
	ip := fmt.Sprintf("100.64.%d.%d", (service.ID%254)+1, (service.ID%254)+100)
	service.TailscaleIP = ip
	service.Exposed = true

	if err := s.db.Save(&service).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to expose service",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service exposed successfully",
		"data":    service,
	})
}

// handleUnexposeK8sService 取消暴露Kubernetes Service
func (s *UnlimitedControlServer) handleUnexposeK8sService(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid service ID",
		})
		return
	}

	var service K8sService
	if err := s.db.First(&service, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s service not found",
		})
		return
	}

	service.TailscaleIP = ""
	service.Exposed = false

	if err := s.db.Save(&service).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to unexpose service",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service unexposed successfully",
		"data":    service,
	})
}

// handleGetServicesByCluster 获取集群的Service列表
func (s *UnlimitedControlServer) handleGetServicesByCluster(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("cluster_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var services []K8sService
	if err := s.db.Where("cluster_id = ?", uint(clusterID)).Order("created_at DESC").Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch cluster services",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    services,
		"total":   len(services),
	})
}

// ===== 节点管理处理函数 =====

// handleListK8sNodes 获取Kubernetes Node列表
func (s *UnlimitedControlServer) handleListK8sNodes(c *gin.Context) {
	var nodes []K8sNode

	query := s.db.Model(&K8sNode{}).Preload("Cluster").Preload("Device")

	// 支持按集群过滤
	if clusterID := c.Query("cluster_id"); clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}

	if err := query.Order("created_at DESC").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch K8s nodes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
		"total":   len(nodes),
	})
}

// handleGetK8sNode 获取单个Kubernetes Node
func (s *UnlimitedControlServer) handleGetK8sNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid node ID",
		})
		return
	}

	var node K8sNode
	if err := s.db.Preload("Cluster").Preload("Device").First(&node, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s node not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    node,
	})
}

// handleRegisterK8sNode 注册Kubernetes Node到Tailscale
func (s *UnlimitedControlServer) handleRegisterK8sNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid node ID",
		})
		return
	}

	var node K8sNode
	if err := s.db.First(&node, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s node not found",
		})
		return
	}

	if node.TailscaleIP != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Node is already registered",
		})
		return
	}

	// 分配Tailscale IP
	ip := fmt.Sprintf("100.64.%d.%d", (node.ID%254)+1, (node.ID%254)+200)
	node.TailscaleIP = ip

	if err := s.db.Save(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to register node",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Node registered successfully",
		"data":    node,
	})
}

// handleUnregisterK8sNode 取消注册Kubernetes Node
func (s *UnlimitedControlServer) handleUnregisterK8sNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid node ID",
		})
		return
	}

	var node K8sNode
	if err := s.db.First(&node, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "K8s node not found",
		})
		return
	}

	node.TailscaleIP = ""
	node.DeviceID = 0

	if err := s.db.Save(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to unregister node",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Node unregistered successfully",
		"data":    node,
	})
}

// handleGetNodesByCluster 获取集群的Node列表
func (s *UnlimitedControlServer) handleGetNodesByCluster(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("cluster_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid cluster ID",
		})
		return
	}

	var nodes []K8sNode
	if err := s.db.Where("cluster_id = ?", uint(clusterID)).Order("created_at DESC").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch cluster nodes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
		"total":   len(nodes),
	})
}

// ===== 状态和监控处理函数 =====

// handleGetK8sStatus 获取Kubernetes状态
func (s *UnlimitedControlServer) handleGetK8sStatus(c *gin.Context) {
	var status struct {
		TotalClusters     int64 `json:"total_clusters"`
		ConnectedClusters int64 `json:"connected_clusters"`
		TotalPods         int64 `json:"total_pods"`
		RunningPods       int64 `json:"running_pods"`
		TotalServices     int64 `json:"total_services"`
		ExposedServices   int64 `json:"exposed_services"`
		TotalNodes        int64 `json:"total_nodes"`
		ReadyNodes        int64 `json:"ready_nodes"`
	}

	// 总集群数
	s.db.Model(&K8sCluster{}).Count(&status.TotalClusters)

	// 已连接集群数
	s.db.Model(&K8sCluster{}).Where("status = ?", "connected").Count(&status.ConnectedClusters)

	// 总Pod数
	s.db.Model(&K8sPod{}).Count(&status.TotalPods)

	// 运行中Pod数
	s.db.Model(&K8sPod{}).Where("phase = ?", "Running").Count(&status.RunningPods)

	// 总Service数
	s.db.Model(&K8sService{}).Count(&status.TotalServices)

	// 已暴露Service数
	s.db.Model(&K8sService{}).Where("exposed = ?", true).Count(&status.ExposedServices)

	// 总Node数
	s.db.Model(&K8sNode{}).Count(&status.TotalNodes)

	// 就绪Node数
	s.db.Model(&K8sNode{}).Where("ready = ?", true).Count(&status.ReadyNodes)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// handleGetK8sMetrics 获取Kubernetes指标
func (s *UnlimitedControlServer) handleGetK8sMetrics(c *gin.Context) {
	var metrics struct {
		PodsByPhase     map[string]int64 `json:"pods_by_phase"`
		ServicesByType  map[string]int64 `json:"services_by_type"`
		NodesByStatus   map[string]int64 `json:"nodes_by_status"`
		ClustersByStatus map[string]int64 `json:"clusters_by_status"`
	}

	// Pod按状态统计
	metrics.PodsByPhase = make(map[string]int64)
	var podPhases []struct {
		Phase string
		Count int64
	}
	s.db.Model(&K8sPod{}).Select("phase, count(*) as count").Group("phase").Scan(&podPhases)
	for _, p := range podPhases {
		metrics.PodsByPhase[p.Phase] = p.Count
	}

	// Service按类型统计
	metrics.ServicesByType = make(map[string]int64)
	var serviceTypes []struct {
		Type  string
		Count int64
	}
	s.db.Model(&K8sService{}).Select("type, count(*) as count").Group("type").Scan(&serviceTypes)
	for _, st := range serviceTypes {
		metrics.ServicesByType[st.Type] = st.Count
	}

	// Node按状态统计
	metrics.NodesByStatus = make(map[string]int64)
	var readyCount, notReadyCount int64
	s.db.Model(&K8sNode{}).Where("ready = ?", true).Count(&readyCount)
	s.db.Model(&K8sNode{}).Where("ready = ?", false).Count(&notReadyCount)
	metrics.NodesByStatus["Ready"] = readyCount
	metrics.NodesByStatus["NotReady"] = notReadyCount

	// 集群按状态统计
	metrics.ClustersByStatus = make(map[string]int64)
	var clusterStatuses []struct {
		Status string
		Count  int64
	}
	s.db.Model(&K8sCluster{}).Select("status, count(*) as count").Group("status").Scan(&clusterStatuses)
	for _, cs := range clusterStatuses {
		metrics.ClustersByStatus[cs.Status] = cs.Count
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// handleGetK8sEvents 获取Kubernetes事件
func (s *UnlimitedControlServer) handleGetK8sEvents(c *gin.Context) {
	// 模拟事件数据
	events := []map[string]interface{}{
		{
			"id":         1,
			"type":       "Normal",
			"reason":     "Created",
			"message":    "Pod nginx-deployment-xxx created",
			"object":     "Pod/nginx-deployment-xxx",
			"namespace":  "default",
			"timestamp":  time.Now().Add(-5 * time.Minute),
		},
		{
			"id":         2,
			"type":       "Normal",
			"reason":     "Started",
			"message":    "Container nginx started",
			"object":     "Pod/nginx-deployment-xxx",
			"namespace":  "default",
			"timestamp":  time.Now().Add(-4 * time.Minute),
		},
		{
			"id":         3,
			"type":       "Warning",
			"reason":     "FailedMount",
			"message":    "Unable to mount volumes for pod",
			"object":     "Pod/app-deployment-yyy",
			"namespace":  "production",
			"timestamp":  time.Now().Add(-2 * time.Minute),
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"total":   len(events),
	})
}

// ===== 网络策略管理处理函数 =====

// handleListK8sNetworkPolicies 获取网络策略列表
func (s *UnlimitedControlServer) handleListK8sNetworkPolicies(c *gin.Context) {
	var policies []K8sNetworkPolicy

	query := s.db.Model(&K8sNetworkPolicy{}).Preload("Cluster")

	// 支持按集群过滤
	if clusterID := c.Query("cluster_id"); clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}

	// 支持按命名空间过滤
	if namespace := c.Query("namespace"); namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}

	if err := query.Order("created_at DESC").Find(&policies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch network policies",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policies,
		"total":   len(policies),
	})
}

// handleCreateK8sNetworkPolicy 创建网络策略
func (s *UnlimitedControlServer) handleCreateK8sNetworkPolicy(c *gin.Context) {
	var req K8sNetworkPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查集群是否存在
	var cluster K8sCluster
	if err := s.db.First(&cluster, req.ClusterID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Cluster not found",
		})
		return
	}

	// 验证策略类型
	validTypes := []string{"Ingress", "Egress", "Both"}
	isValidType := false
	for _, t := range validTypes {
		if req.PolicyType == t {
			isValidType = true
			break
		}
	}

	if !isValidType {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy type. Must be Ingress, Egress, or Both",
		})
		return
	}

	// 设置默认值
	if req.Namespace == "" {
		req.Namespace = "default"
	}
	req.Enabled = true

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create network policy",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetK8sNetworkPolicy 获取网络策略
func (s *UnlimitedControlServer) handleGetK8sNetworkPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var policy K8sNetworkPolicy
	if err := s.db.Preload("Cluster").First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Network policy not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policy,
	})
}

// handleUpdateK8sNetworkPolicy 更新网络策略
func (s *UnlimitedControlServer) handleUpdateK8sNetworkPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var req K8sNetworkPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var policy K8sNetworkPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Network policy not found",
		})
		return
	}

	// 更新字段
	policy.Name = req.Name
	policy.Namespace = req.Namespace
	policy.PolicyType = req.PolicyType
	policy.PodSelector = req.PodSelector
	policy.Ingress = req.Ingress
	policy.Egress = req.Egress

	if err := s.db.Save(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update network policy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policy,
	})
}

// handleDeleteK8sNetworkPolicy 删除网络策略
func (s *UnlimitedControlServer) handleDeleteK8sNetworkPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var policy K8sNetworkPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Network policy not found",
		})
		return
	}

	if err := s.db.Delete(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete network policy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Network policy deleted successfully",
	})
}

// handleEnableK8sNetworkPolicy 启用网络策略
func (s *UnlimitedControlServer) handleEnableK8sNetworkPolicy(c *gin.Context) {
	s.toggleK8sNetworkPolicy(c, true)
}

// handleDisableK8sNetworkPolicy 禁用网络策略
func (s *UnlimitedControlServer) handleDisableK8sNetworkPolicy(c *gin.Context) {
	s.toggleK8sNetworkPolicy(c, false)
}

// toggleK8sNetworkPolicy 切换网络策略状态
func (s *UnlimitedControlServer) toggleK8sNetworkPolicy(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var policy K8sNetworkPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Network policy not found",
		})
		return
	}

	policy.Enabled = enabled
	if err := s.db.Save(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update network policy status",
		})
		return
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Network policy %s successfully", action),
		"data":    policy,
	})
}

// ===== Operator配置管理处理函数 =====

// handleListK8sOperatorConfigs 获取Operator配置列表
func (s *UnlimitedControlServer) handleListK8sOperatorConfigs(c *gin.Context) {
	var configs []K8sOperatorConfig

	query := s.db.Model(&K8sOperatorConfig{}).Preload("Cluster")

	// 支持按集群过滤
	if clusterID := c.Query("cluster_id"); clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}

	if err := query.Order("created_at DESC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch operator configs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    configs,
		"total":   len(configs),
	})
}

// handleCreateK8sOperatorConfig 创建Operator配置
func (s *UnlimitedControlServer) handleCreateK8sOperatorConfig(c *gin.Context) {
	var req K8sOperatorConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查集群是否存在
	var cluster K8sCluster
	if err := s.db.First(&cluster, req.ClusterID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Cluster not found",
		})
		return
	}

	// 设置默认值
	if req.OperatorImage == "" {
		req.OperatorImage = "tailscale/operator:latest"
	}
	if req.OperatorVersion == "" {
		req.OperatorVersion = "1.0.0"
	}
	if req.Namespace == "" {
		req.Namespace = "tailscale-system"
	}
	if req.ServiceAccount == "" {
		req.ServiceAccount = "tailscale-operator"
	}
	if req.ClusterRole == "" {
		req.ClusterRole = "tailscale-operator"
	}
	if req.SyncInterval <= 0 {
		req.SyncInterval = 300 // 5分钟
	}
	if req.LogLevel == "" {
		req.LogLevel = "info"
	}
	req.Enabled = true

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create operator config",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetK8sOperatorConfig 获取Operator配置
func (s *UnlimitedControlServer) handleGetK8sOperatorConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var config K8sOperatorConfig
	if err := s.db.Preload("Cluster").First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Operator config not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// handleUpdateK8sOperatorConfig 更新Operator配置
func (s *UnlimitedControlServer) handleUpdateK8sOperatorConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var req K8sOperatorConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var config K8sOperatorConfig
	if err := s.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Operator config not found",
		})
		return
	}

	// 更新字段
	config.OperatorImage = req.OperatorImage
	config.OperatorVersion = req.OperatorVersion
	config.Namespace = req.Namespace
	config.ServiceAccount = req.ServiceAccount
	config.ClusterRole = req.ClusterRole
	config.ConfigMap = req.ConfigMap
	config.Secret = req.Secret
	config.AutoSync = req.AutoSync
	config.SyncInterval = req.SyncInterval
	config.LogLevel = req.LogLevel

	if err := s.db.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update operator config",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// handleDeleteK8sOperatorConfig 删除Operator配置
func (s *UnlimitedControlServer) handleDeleteK8sOperatorConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var config K8sOperatorConfig
	if err := s.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Operator config not found",
		})
		return
	}

	if err := s.db.Delete(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete operator config",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Operator config deleted successfully",
	})
}

// handleDeployK8sOperator 部署Operator
func (s *UnlimitedControlServer) handleDeployK8sOperator(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var config K8sOperatorConfig
	if err := s.db.Preload("Cluster").First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Operator config not found",
		})
		return
	}

	// 生成Operator部署清单
	km := NewK8sManager(s)
	manifest, err := km.GenerateOperatorManifest(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate operator manifest: " + err.Error(),
		})
		return
	}

	// 在实际实现中，这里会使用Kubernetes客户端部署Operator
	// 目前返回生成的清单供用户手动部署
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Operator manifest generated successfully",
		"data": map[string]interface{}{
			"config":   config,
			"manifest": manifest,
			"instructions": "Please apply this manifest to your Kubernetes cluster using: kubectl apply -f manifest.yaml",
		},
	})
}

// handleUndeployK8sOperator 卸载Operator
func (s *UnlimitedControlServer) handleUndeployK8sOperator(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var config K8sOperatorConfig
	if err := s.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Operator config not found",
		})
		return
	}

	// 在实际实现中，这里会使用Kubernetes客户端卸载Operator
	// 目前提供卸载指令
	undeployInstructions := fmt.Sprintf(`
To undeploy the Tailscale Operator from cluster "%s":

1. Delete the operator deployment:
   kubectl delete deployment tailscale-operator -n %s

2. Delete the service account and RBAC:
   kubectl delete serviceaccount %s -n %s
   kubectl delete clusterrole %s
   kubectl delete clusterrolebinding %s-binding

3. Delete the namespace (if dedicated):
   kubectl delete namespace %s

4. Clean up any remaining resources:
   kubectl delete secret tailscale-auth -n %s
`, config.Cluster.Name, config.Namespace, config.ServiceAccount, config.Namespace,
		config.ClusterRole, config.ClusterRole, config.Namespace, config.Namespace)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Operator undeploy instructions generated",
		"data": map[string]interface{}{
			"config":       config,
			"instructions": undeployInstructions,
		},
	})
}
