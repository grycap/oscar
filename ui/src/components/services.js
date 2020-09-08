import JSZip from "jszip";
import JSZipUtils from "jszip-utils"
export default {
    data: () => {
		return {
            api: '',
            minioClient: '',  
            username_auth:'',
            password_auth:''        
		}
    },
    created(){
        this.username_auth = localStorage.getItem("user");
        this.password_auth = localStorage.getItem("password");
        var minio_endpoint = localStorage.getItem("endpoint");
        var minio_port = localStorage.getItem("port");
        // var minio_useSSL = localStorage.getItem("useSSL");
        var minio_accessKey = localStorage.getItem("accessKey");
        var minio_secretKey = localStorage.getItem("secretKey");



        var Minio = require('minio')
        this.minioClient = new Minio.Client({
            endPoint: minio_endpoint,    
            port: parseInt(minio_port),   
            useSSL: true,
            accessKey: minio_accessKey,
            secretKey: minio_secretKey
        });
        this.minioClient.setRequestOptions({rejectUnauthorized: false})

    },
    methods: {
        //ApiCalls
        checkLoginCall(params,callBackHandler){
            var _this = this
            axios({
                method: 'get',
                url: '/system/info',
                auth: {
                    username: params.user,
                    password: params.password
                }
            }).then(function (response) {
                // _this.username_auth = params.user
                // _this.password_auth = params.password
                callBackHandler(response.status);
            }.bind(this)).catch(function (error) {
                callBackHandler(error.response.status);
            })

        },
        listServicesCall(callBackHandler) {
            axios({
                method: 'get',
                url: '/system/services',
                auth: {
                    username: this.username_auth,
                    password: this.password_auth
                }
            }).then(function (response) {
                callBackHandler(response);
            }.bind(this)).catch(function (error) {
                callBackHandler(error);
            })
        },
        deleteServiceCall(params, callBackHandler) {
            axios({
                method: 'delete',
                url: '/system/services/'+params,
                auth: {
                    username: this.username_auth,
                    password: this.password_auth
                },
                data:params,
            }).then(function (response) {
                callBackHandler(response);
            }.bind(this)).catch(function (error) {
                callBackHandler(error);
            })
        },
        listJobsCall(serviceName,callBackHandler) {
            axios({
                method: 'get',
                url: '/system/logs/'+serviceName,
                auth: {
                    username: this.username_auth,
                    password: this.password_auth
                }
            }).then(function (response) {
                callBackHandler(response.data);
            }.bind(this)).catch(function (error) {
                callBackHandler(error);
            })
        },
        deleteJobCall(params, callBackHandler) {
            axios({
                method: 'delete',
                url:  '/system/logs/'+params.serviceName+'/'+params.jobName,
                auth: {
                    username: this.username_auth,
                    password: this.password_auth
                },
                data:params,
            }).then(function (response) {
                callBackHandler("success");
            }.bind(this)).catch(function (error) {
                callBackHandler(error);
            })
        },
        listJobNameCall(params,callBackHandler) {
            axios({
                method: 'get',
                url: '/system/logs/'+params.serviceName+'/'+params.jobName,
                auth: {
                    username: this.username_auth,
                    password: this.password_auth
                }
            }).then(function (response) {
                callBackHandler(response.data);
            }.bind(this)).catch(function (error) {
                callBackHandler(error);
            })
        },
        deleteAllJobCall(params,callBackHandler) {
            axios({
                method: 'delete',
                url: '/system/logs/'+params.serviceName+'?all='+params.all,
                auth: {
                    username: this.username_auth,
                    password: this.password_auth
                }
            }).then(function (response) {
                callBackHandler(response);
            }.bind(this)).catch(function (error) {
                callBackHandler(error);
            })
        },
        
        createServiceCall(params, callBackHandler){
            axios({
                method: 'post',
                url: '/system/services',
                auth: {
                    username: this.username_auth,
                    password: this.password_auth
                },
                data: params
            }).then(function(response){
                callBackHandler(response)
            }).catch(function(error){
                callBackHandler(error)
            })
        },

        editServiceCall(params, callBackHandler){
            axios({
                method: 'put',
                url: '/system/services',
                auth: {
                    username: this.username_auth,
                    password: this.password_auth
                },
                data: params
            }).then(function(response){
                callBackHandler(response)
            }).catch(function(error){
                callBackHandler(error)
            })
        },

        //******Minio's Call********/
        
        getBucketListCall(callBackHandler){
            this.minioClient.listBuckets((err, buckets) => {
                if (err) {
                    callBackHandler(err)
                }else{
                    callBackHandler(buckets)
                }
                
            })
        },

        createBucketCall(params,callBackHandler){
            this.minioClient.makeBucket(params.name, function(err, exists) {
                if (err) {
                    callBackHandler(err)
                }else{
                    callBackHandler("success")       
                }         
                    
            })
        },

        bucketExistCall(params,callBackHandler){
            this.minioClient.bucketExists(params.name, function(err, exists) {
                if (err){
                    callBackHandler(err)
                    window.getApp.$emit('APP_SHOW_SNACKBAR', {
                        text: err.message,
                        color: 'error'
                    })
                }else{
                    callBackHandler('success')
                }        
                    
            })
        },

        getBucketFilesCall(params, callBackHandler){
            let stream = this.minioClient.listObjects(params.name, params.prefix, true,) 
            var funct = {
                err : "",
                files: []
            };
            stream.on('data', function(obj) 
            {   
                funct.files.push(obj);
                
            })    
            stream.on('error', function(err) 
            {       
                funct["err"] = err;
            })
            stream.on('end', function(e) 
            {       
                callBackHandler(funct);
            })
        },
        previewFileCall(params,callBackHandler){  
            this.minioClient.presignedUrl('GET',params.bucketName, params.fileName, 30000, function(err, presignedUrl) {
                if (err){
                    callBackHandler(err)
                }else{
                    callBackHandler(presignedUrl)
                   
                }
               
              })
            
        },
        urlToPromise(url) {
            return new Promise(function(resolve, reject) {
                JSZipUtils.getBinaryContent(url, function (err, data) {
                    if(err) {
                        reject(err);
                    } else {
                        resolve(data);
                    }
                });
            });
        },
        downloadFileCall(params,callBackHandler){   
            var _this = this     
            if (params.select == 1){
                this.minioClient.presignedGetObject(params.bucketName, params.fileName[0], 1500, function(err, presignedUrl) {
                    if (err){
                        callBackHandler(err)
                    }else{
                        axios({url:presignedUrl,method:'GET',responseType: params.response_type})
                        .then(response => {
                            callBackHandler(response)
                        })
                    }
                })
            }else {
                    let zip = new JSZip();
                    let folder = zip.folder('collection');
                    for (let i = 0; i < params.select; i++) {
                        this.minioClient.presignedGetObject(params.bucketName, params.fileName[i], 30000, function(err, presignedUrl) {
                             if (err){
                                callBackHandler(err)
                             }else{
                                // Fetch the image and parse the response stream as a blob
                                var name = params.fileName[i].substr(params.fileName[i].lastIndexOf('/') + 1);
                                folder.file(name, _this.urlToPromise(presignedUrl), {binary:true});
                            }
                        })                                           
                    }

                    callBackHandler(folder)
                }
        },
        uploadFileCall(params, callBackHandler){
            this.minioClient.presignedPutObject(params.bucketName, params.file_name, 24*60*60, function(err, presignedUrl) {
                if (err){
                    console.log(err)  
                }else{
                    fetch(presignedUrl, {
                        method: 'PUT',
                        body: params.file
             
                    }).then(() => {
                       callBackHandler('uploaded')
                    }).catch((e) => {
                       callBackHandler(e)
                    });
                } 
                
            })
        },
        removeFileCall(params,callBackHandler){
            var objectList = [];
            objectList = params.fileName
            for(var i=0; i < objectList.length; i++) {
                this.minioClient.removeObject(params.bucketName, objectList[i], function(err, exists) {
                    if (err){ 
                        callBackHandler(error)          
                    }else{
                        callBackHandler("success");        
                    }
                        
                })
            }

        },

        removeBucketCall(params,callBackHandler){

            var objectsList = []
            var _this = this

            // List all object paths in bucket my-bucketname.
            var objectsStream = this.minioClient.listObjects(params, '', true)

            objectsStream.on('data', function(obj) {
            objectsList.push(obj.name);
            })

            objectsStream.on('error', function(e) {
            console.log(e);
            })

            objectsStream.on('end', function() {

            _this.minioClient.removeObjects(params,objectsList, function(e) {
                if (e) {
                    return console.log('Unable to remove Objects ',e)
                }
                _this.minioClient.removeBucket(params, function(err, exists) {    
                    console.log(err)
                    if (err){
                        callBackHandler(err)
                    }else{
                        callBackHandler('success');        
                    }             
                        
                })
                console.log('Removed the objects successfully')
            })

            })
            
        },
    },
}