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
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/cdmi-client-go"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	defaultMemory   = "256Mi"
	defaultCPU      = "0.2"
	defaultLogLevel = "INFO"
	createPath      = "/system/services"
)

var errInput = errors.New("unrecognized input (valid inputs are MinIO and dCache)")

// Custom logger
var createLogger = log.New(os.Stdout, "[CREATE-HANDLER] ", log.Flags())
var isAdminUser = false

// MakeCreateHandler godoc
// @Summary Create service
// @Description Create a new OSCAR service definition.
// @Tags services
// @Accept json
// @Produce json
// @Param service body types.Service true "Service definition"
// @Success 201 {string} string "Created"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services [post]
func MakeCreateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service types.Service
		isAdminUser = false
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			isAdminUser = true
			service.Owner = types.DefaultOwner
			createLogger.Printf("Creating service '%s' for user '%s'", service.Name, service.Owner)
		}
		if err := c.ShouldBindJSON(&service); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
		}
		service.AllowedUsers = sanitizeUsers(service.AllowedUsers)
		service.Script = utils.NormalizeLineEndings(service.Script)

		// Check service values and set defaults
		checkValues(&service, cfg)
		// Check if users in allowed_users have a MinIO associated user
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)

		// Service is created by an EGI user
		var uid string
		var err error
		var mc *auth.MultitenancyConfig
		namespace := cfg.ServicesNamespace
		if !isAdminUser {
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}
			if uid == "" {
				c.String(http.StatusInternalServerError, fmt.Sprintln("Couldn't find user identification"))
				return
			}
			// Set UID from owner
			service.Owner = uid
			createLogger.Printf("Creating service '%s' for user '%s'", service.Name, service.Owner)

			mc, err = auth.GetMultitenancyConfigFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}

			full_uid := auth.FormatUID(uid)
			// Check if the service VO is present on the cluster VO's and if the user creating the service is enrrolled in such
			if service.VO != "" {
				for _, vo := range cfg.OIDCGroups {
					if vo == service.VO {
						err := checkIdentity(&service, authHeader)
						if err != nil {
							c.String(http.StatusBadRequest, fmt.Sprintln(err))
							return
						}
						break
					}
				}
			} else {
				if len(cfg.OIDCGroups) != 0 {
					var notFound bool = true
					for _, vo := range cfg.OIDCGroups {
						service.VO = vo
						err := checkIdentity(&service, authHeader)
						if err == nil {
							notFound = false
							break
						}
					}
					if notFound {
						c.String(http.StatusBadRequest, fmt.Sprintln("service must be part of one of the following VO: ", cfg.OIDCGroups))
						return
					}

				}
			}

			ownerOnList := false
			namespace, err = utils.EnsureUserNamespace(c.Request.Context(), back.GetKubeClientset(), cfg, uid)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("error ensuring namespace for user %s: %v", uid, err))
				return
			}
			if err := mc.EnsureSecretInNamespace(uid, namespace); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("error ensuring credentials for user %s: %v", uid, err))
				return
			}

			if len(service.AllowedUsers) > 0 && strings.ToUpper(service.IsolationLevel) == types.IsolationLevelUser {
				for _, in := range service.Input {
					_, provName := getProviderInfo(in.Provider)

					// Only allow input from MinIO and dCache
					if provName == types.MinIOName {
						path := strings.Trim(in.Path, "/")
						splitPath := strings.SplitN(path, "/", 2)
						// If AllowedUsers is empty don't add uid
						service.Labels["uid"] = full_uid[:10]
						var userBucket string
						for _, u := range service.AllowedUsers {
							// Check if the uid's from allowed_users have and asociated MinIO user
							// and create it if not
							if !mc.UserExists(u) {
								sk, _ := auth.GenerateRandomKey(8)
								cmuErr := minIOAdminClient.CreateMinIOUser(u, sk)
								if cmuErr != nil {
									log.Printf("error creating MinIO user for user %s: %v", u, cmuErr)
								}
								csErr := mc.CreateSecretForOIDC(u, sk)
								if csErr != nil {
									log.Printf("error creating secret for user %s: %v", u, csErr)
								}
							}
							// Fill the list of private buckets to be used on users buckets isolation
							// Check the uid of the owner is on the allowed_users list
							if u == service.Owner {
								ownerOnList = true
							}
							// Fill the list of private buckets to create
							userBucket = splitPath[0] + "-" + u[:10]
							service.BucketList = append(service.BucketList, userBucket)
							if err := mc.EnsureSecretInNamespace(u, namespace); err != nil {
								c.String(http.StatusInternalServerError, fmt.Sprintf("error ensuring credentials for user %s: %v", u, err))
								return
							}
						}

						if !ownerOnList {
							service.AllowedUsers = append(service.AllowedUsers, uid)
						}
					}
				}
			}
		}
		if service.Namespace == "" {
			service.Namespace = namespace
		}
		if !isAdminUser {
			if err := mc.EnsureSecretInNamespace(service.Owner, service.Namespace); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("error ensuring credentials for user %s: %v", service.Owner, err))
				return
			}
		}
		if len(service.Environment.Secrets) > 0 {
			if utils.SecretExists(service.Name, service.Namespace, back.GetKubeClientset()) {
				c.String(http.StatusConflict, "A secret with the given name already exists")
			}
			secretsErr := utils.CreateSecret(service.Name, service.Namespace, service.Environment.Secrets, back.GetKubeClientset())
			if secretsErr != nil {
				c.String(http.StatusConflict, "Error creating secrets for service: %v", secretsErr)
			}

			// Empty the secrets content from the Configmap
			for secretKey := range service.Environment.Secrets {
				service.Environment.Secrets[secretKey] = ""
			}
		}

		// Create service
		if err := back.CreateService(service); err != nil {
			// Check if error is caused because the service name provided already exists
			if k8sErrors.IsAlreadyExists(err) {
				c.String(http.StatusConflict, "A service with the provided name already exists")
			} else {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating the service: %v", err))
			}
			return
		}

		// Register minio webhook and restart the server
		if err := registerMinIOWebhook(service.Name, service.Token, service.StorageProviders.MinIO[types.DefaultProvider], cfg); err != nil {
			derr := back.DeleteService(service)
			if derr != nil {
				log.Printf("Error deleting service: %v\n", derr)
			}
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		var buckets []utils.MinIOBucket
		if buckets, err = createBuckets(&service, cfg, minIOAdminClient, false); err != nil {
			if err == errInput {
				c.String(http.StatusBadRequest, err.Error())
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			derr := back.DeleteService(service)
			if derr != nil {
				log.Printf("Error deleting service: %v\n", derr)
			}

			if !strings.Contains(err.Error(), " already exists") {
				bderr := deleteBuckets(&service, cfg, minIOAdminClient)
				if bderr != nil {
					log.Printf("Error deleting buckets: %v\n", bderr)
				}
			}
			return
		}
		if len(buckets) > 0 {
			for _, b := range buckets {
				// If not specified default visibility is PRIVATE
				if strings.ToLower(service.Visibility) == "" {
					b.Visibility = utils.PRIVATE
				}
				if service.Owner != types.DefaultOwner {
					err := minIOAdminClient.SetPolicies(b)
					if err != nil {
						c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating the service: %v", err))
					}
				}

				ownerName := "oscar"
				if !isAdminUser {
					ownerName = auth.GetUserNameFromContext(c)
					ownerName = utils.RemoveAccents(ownerName)
				}
				// Bucket metadata for filtering
				tags := map[string]string{
					"owner":        uid,
					"from_service": service.Name,
					"owner_name":   ownerName,
				}
				if err := minIOAdminClient.SetTags(b.BucketName, tags); err != nil {
					c.String(http.StatusBadRequest, fmt.Sprintf("Error tagging bucket: %v", err))
				}
			}
		}

		// Add Yunikorn queue if enabled
		if cfg.YunikornEnable {
			if err := utils.AddYunikornQueue(cfg, back.GetKubeClientset(), &service); err != nil {
				log.Println(err.Error())
			}
		}

		createLogger.Printf("%s | %v | %s | %s | %s", "POST", 200, createPath, service.Name, uid)
		c.Status(http.StatusCreated)
	}
}

func checkValues(service *types.Service, cfg *types.Config) {
	// Add default values for Memory and CPU if they are not set
	// Do not validate, Kubernetes client throws an error if they are not correct
	if service.Memory == "" {
		service.Memory = defaultMemory
	}
	if service.CPU == "" {
		service.CPU = defaultCPU
	}
	// Check if visibility has been set. If not set default private.
	if service.Visibility == "" {
		service.Visibility = utils.PRIVATE
	}

	// Validate logLevel (Python logging levels for faas-supervisor)
	service.LogLevel = strings.ToUpper(service.LogLevel)
	switch service.LogLevel {
	case "NOTSET", "DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL":
	default:
		service.LogLevel = defaultLogLevel
	}

	// Add default Labels
	if service.Labels == nil {
		service.Labels = make(map[string]string)
	}
	service.Labels[types.ServiceLabel] = service.Name
	service.Labels[types.YunikornApplicationIDLabel] = service.Name
	service.Labels[types.YunikornQueueLabel] = fmt.Sprintf("%s.%s.%s", types.YunikornRootQueue, types.YunikornOscarQueue, service.Name)

	// Create default annotations map
	if service.Annotations == nil {
		service.Annotations = make(map[string]string)
	}

	// Add the default MinIO provider without credentials
	defaultMinIOInstanceInfo := &types.MinIOProvider{
		Endpoint:  cfg.MinIOProvider.Endpoint,
		Verify:    cfg.MinIOProvider.Verify,
		AccessKey: "hidden",
		SecretKey: "hidden",
		Region:    cfg.MinIOProvider.Region,
	}

	if service.StorageProviders != nil {
		if service.StorageProviders.MinIO != nil {
			service.StorageProviders.MinIO[types.DefaultProvider] = defaultMinIOInstanceInfo
		} else {
			service.StorageProviders.MinIO = map[string]*types.MinIOProvider{
				types.DefaultProvider: defaultMinIOInstanceInfo,
			}
		}
	} else {
		service.StorageProviders = &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{
				types.DefaultProvider: defaultMinIOInstanceInfo,
			},
		}
	}

	// Generate a new access token
	service.Token = utils.GenerateToken()
}

func createBuckets(service *types.Service, cfg *types.Config, minIOAdminClient *utils.MinIOAdminClient, isUpdate bool) ([]utils.MinIOBucket, error) {
	var s3Client *s3.S3
	var cdmiClient *cdmi.Client
	var provName, provID string
	var minIOBuckets []utils.MinIOBucket

	// ========== CREATE INPUT BUCKETS ==========
	for _, in := range service.Input {
		provID, provName = getProviderInfo(in.Provider)

		// Only allow input from MinIO and dCache
		if provName != types.MinIOName && provName != types.WebDavName && provName != types.RucioName {
			return nil, errInput
		}

		// If the provider is WebDav (dCache) skip bucket creation
		if provName == types.WebDavName || provName == types.RucioName {
			continue
		}

		// Check if the provider identifier is defined in StorageProviders
		if !isStorageProviderDefined(provName, provID, service.StorageProviders) {
			return nil, fmt.Errorf("the StorageProvider \"%s.%s\" is not defined", provName, provID)
		}

		// Check if the input provider is the defined in the server config
		if provID != types.DefaultProvider {
			if !reflect.DeepEqual(*cfg.MinIOProvider, *service.StorageProviders.MinIO[provID]) {
				return nil, fmt.Errorf("the provided MinIO server \"%s\" is not the configured in OSCAR", service.StorageProviders.MinIO[provID].Endpoint)
			}
		}

		// Use admin MinIO client for the bucket creation
		s3Client = cfg.MinIOProvider.GetS3Client()

		path := strings.Trim(in.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)
		folderKey := fmt.Sprintf("%s/", splitPath[1])

		err := minIOAdminClient.CreateS3PathWithWebhook(s3Client, splitPath, service.GetMinIOWebhookARN(), false)

		if err != nil && !isUpdate {
			return nil, err
		}

		minIOBuckets = append(minIOBuckets, utils.MinIOBucket{
			BucketName:   splitPath[0],
			AllowedUsers: service.AllowedUsers,
			Visibility:   service.Visibility,
			Owner:        service.Owner,
		})
		// Create buckets for services with isolation level
		if strings.ToUpper(service.IsolationLevel) == types.IsolationLevelUser && len(service.BucketList) > 0 {
			for i, b := range service.BucketList {
				// Create a bucket for each allowed user if allowed_users is not empty
				err = minIOAdminClient.CreateS3PathWithWebhook(s3Client, []string{b, folderKey}, service.GetMinIOWebhookARN(), false)
				if err != nil && isUpdate {
					continue
				} else {
					if err != nil {
						return nil, err
					}
				}
				// Create bucket policy
				if !isAdminUser {
					err = minIOAdminClient.CreateAddPolicy(b, service.AllowedUsers[i], utils.ALL_ACTIONS, false)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	// ========== CREATE OUTPUT BUCKETS ==========
	// Create output buckets
	for _, out := range service.Output {
		provID, provName = getProviderInfo(out.Provider)
		// Check if the provider identifier is defined in StorageProviders
		if !isStorageProviderDefined(provName, provID, service.StorageProviders) {
			return nil, fmt.Errorf("the StorageProvider \"%s.%s\" is not defined", provName, provID)
		}

		path := strings.Trim(out.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)
		folderKey := fmt.Sprintf("%s/", splitPath[1])

		switch provName {
		case types.MinIOName, types.S3Name:
			// Use the appropriate client
			if provName == types.MinIOName {
				if provID == types.DefaultProvider {
					s3Client = cfg.MinIOProvider.GetS3Client()
				} else {
					s3Client = service.StorageProviders.MinIO[provID].GetS3Client()

				}
			} else {
				s3Client = service.StorageProviders.S3[provID].GetS3Client()
			}
			var found bool
			for _, b := range minIOBuckets {
				if b.BucketName == splitPath[0] {
					found = true
					break
				}
			}
			if !found {
				// If the bucket hasn't been created on de input loop create it
				minIOBuckets = append(minIOBuckets, utils.MinIOBucket{
					BucketName:   splitPath[0],
					AllowedUsers: service.AllowedUsers,
					Visibility:   service.Visibility,
					Owner:        service.Owner})
				err := minIOAdminClient.CreateS3Path(s3Client, splitPath, false)
				if err != nil && !isUpdate {
					return nil, err
				}
			} else {
				// If the bucket is created on the previous loop, add output folders
				err := minIOAdminClient.CreateS3Path(s3Client, splitPath, true)
				if err != nil && !isUpdate {
					return nil, err
				}
			}

			if strings.ToUpper(service.IsolationLevel) == types.IsolationLevelUser && len(service.BucketList) > 0 {
				for _, b := range service.BucketList {
					err := minIOAdminClient.CreateS3Path(s3Client, []string{b, folderKey}, true)
					if err != nil && !isUpdate {
						return nil, err
					}
				}
			}

		case types.OnedataName:
			cdmiClient = service.StorageProviders.Onedata[provID].GetCDMIClient()
			err := cdmiClient.CreateContainer(fmt.Sprintf("%s/%s", service.StorageProviders.Onedata[provID].Space, path), true)
			if err != nil {
				if err == cdmi.ErrBadRequest {
					log.Printf("Error creating \"%s\" folder in Onedata. Error: %v\n", path, err)
				} else {
					return nil, fmt.Errorf("error connecting to Onedata's Oneprovider \"%s\". Error: %v", service.StorageProviders.Onedata[provID].OneproviderHost, err)
				}
			}
		}
	}

	if service.Mount.Provider != "" {
		provID, provName = getProviderInfo(service.Mount.Provider)
		if provName == types.MinIOName {
			// Check if the provider identifier is defined in StorageProviders
			if !isStorageProviderDefined(provName, provID, service.StorageProviders) {
				return nil, fmt.Errorf("the StorageProvider \"%s.%s\" is not defined", provName, provID)
			}

			path := strings.Trim(service.Mount.Path, " /")
			// Split buckets and folders from path
			splitPath := strings.SplitN(path, "/", 2)

			// Currently only MinIO/S3 are supported
			// Use the appropriate client
			if provName == types.MinIOName && provID == types.DefaultProvider {
				s3Client = cfg.MinIOProvider.GetS3Client()
			} else if provName == types.MinIOName {
				return minIOBuckets, nil
			} else {
				s3Client = service.StorageProviders.S3[provID].GetS3Client()
			}

			// Check if the bucket exists in the service
			var foundInService bool
			for _, b := range minIOBuckets {
				if b.BucketName == splitPath[0] {
					foundInService = true
					break
				}
			}

			// If the bucket exists in the input/output loop don't create it again
			// just create the folder if needed

			if foundInService {
				err := minIOAdminClient.CreateS3Path(s3Client, splitPath, true)
				if err != nil && !isUpdate {
					return nil, err
				}
				return minIOBuckets, nil
			}

			// List buckets to check if the bucket exists in MinIO
			bucketInfo, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
			if err != nil {
				return nil, err
			}

			var foundInMinIO bool
			for _, b := range bucketInfo.Buckets {
				if *b.Name == splitPath[0] {
					foundInMinIO = true
					break
				}
			}
			if foundInMinIO {
				minio := utils.MinIOBucket{
					BucketName: splitPath[0],
					Owner:      service.Owner,
				}
				visibility := minIOAdminClient.GetCurrentResourceVisibility(minio)
				if visibility != utils.PRIVATE {
					return nil, fmt.Errorf("the bucket \"%s\" must be private to be used as mount", minio.BucketName)
				} else {
					err := minIOAdminClient.CreateS3Path(s3Client, splitPath, true)
					minIOBuckets = append(minIOBuckets, utils.MinIOBucket{
						BucketName:   splitPath[0],
						AllowedUsers: service.AllowedUsers,
						Visibility:   service.Visibility,
						Owner:        service.Owner})
					if err != nil && !isUpdate {
						return nil, err
					}
					return minIOBuckets, nil
				}
			}

			// Create mount bucket
			err = minIOAdminClient.CreateS3Path(s3Client, splitPath, false)
			minIOBuckets = append(minIOBuckets, utils.MinIOBucket{
				BucketName:   splitPath[0],
				AllowedUsers: service.AllowedUsers,
				Visibility:   service.Visibility,
				Owner:        service.Owner})
			if err != nil {
				return nil, err
			}

		}
	}

	return minIOBuckets, nil
}

func isStorageProviderDefined(storageName string, storageID string, providers *types.StorageProviders) bool {
	var ok = false
	switch storageName {
	case types.MinIOName:
		_, ok = providers.MinIO[storageID]
	case types.S3Name:
		_, ok = providers.S3[storageID]
	case types.OnedataName:
		_, ok = providers.Onedata[storageID]
	case types.WebDavName:
		_, ok = providers.WebDav[storageID]
	case types.RucioName:
		_, ok = providers.Rucio[storageID]
	}
	return ok
}

func getProviderInfo(rawInfo string) (string, string) {
	var provID, provName string
	// Split input provider
	provSlice := strings.SplitN(strings.TrimSpace(rawInfo), types.ProviderSeparator, 2)
	if len(provSlice) == 1 {
		provName = strings.ToLower(provSlice[0])
		// Set "default" provider ID
		provID = types.DefaultProvider
	} else {
		provName = strings.ToLower(provSlice[0])
		provID = provSlice[1]
	}
	return provID, provName
}

func checkIdentity(service *types.Service, authHeader string) error {
	rawToken := strings.TrimPrefix(authHeader, "Bearer ")
	issuer, err := auth.GetIssuerFromToken(rawToken)
	if err != nil {
		return err
	}
	oidcManager := auth.ClusterOidcManagers[issuer]
	if oidcManager == nil {
		return err
	}
	ui, err := oidcManager.GetUserInfo(rawToken)
	if err != nil {
		return err
	}
	hasVO := oidcManager.UserHasVO(ui, service.VO)

	if !hasVO {
		return fmt.Errorf("this user isn't enrrolled on the vo: %v", service.VO)
	}
	voFirstReplace := strings.Replace(service.VO, "/", "", 1)
	voParse := strings.ReplaceAll(voFirstReplace, "/", "--")
	voParse = strings.ReplaceAll(voParse, " ", "__")
	service.Labels["vo"] = voParse

	return nil
}

func registerMinIOWebhook(name string, token string, minIO *types.MinIOProvider, cfg *types.Config) error {
	minIOAdminClient, err := utils.MakeMinIOAdminClient(cfg)
	if err != nil {
		return fmt.Errorf("the provided MinIO configuration is not valid: %v", err)
	}

	if err := minIOAdminClient.RegisterWebhook(name, token); err != nil {
		return fmt.Errorf("error registering the service's webhook: %v", err)
	}

	return minIOAdminClient.RestartServer()
}
