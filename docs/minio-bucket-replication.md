# MinIO bucket replication

There could be an scenario where you have two OSCAR clusters sharing data and the connection gets lost, so the data generated on the first cluster during the desconection time would get lost as well. 
        
In order to resolve this scenario we propose the use of replicated buckets on MinIO. With this approach you can have two buckets synchronized on different clusters that, in a case where the connection is lost, will be re-synchronized when it is restored.

You can see an example of this scenario on the following diagram, where you have two MinIO instances (each one on a different cluster), and the output of the execution of *service_x* on the source serves as input for the function of *service_y* on the remote cluster.

![minio-replication-diagram](images/minio-bucket-replication/minio-replication-diagram.png)

Here is in more detail the dataflow between buckets:

**MinIO instance source**
- `input`: receives some data and triggers the execution of the *service_x* function.
- `intermediate`: the output of the previous execution is stored on this bucket and synchronized with the intermediate bucket on the remote instance. 

**MinIO instance remote**
- `intermediate`: the synchronized bucket that stores the replicated data and triggers the service function over them.
- `output`: stores the output of the previous execution.

### Considerations

When you create the service on the remote OSCAR cluster, the `intermediate` bucket that is both the replica and input of the service function, will have the webhook event for PUT actions enabled so it can be used as trigger for the function.

Because, as explained below on [Event handling on replication events](#Event-handling-on-replication-events), there are some specific events for replicated buckets, it is important to delete this event webhook so you don't get both events everytime.

```bash!
mc event remove originminio/intermediate arn:aws:sqs::intermediate:webhook --event put
```

## Helm installation

To be able to use replication each minIO instance deployed with helm has to be on distributed mode. This is done by adding the parameters `mode=distributed,replicas=NUM_REPLICAS`.

Here is an example of a local minIO replicated deployment with helm:

```bash!
helm install minio minio/minio --namespace minio --set rootUser=minio,rootPassword=minio123,service.type=NodePort,service.nodePort=30300,consoleService.type=NodePort,consoleService.nodePort=30301,mode=distributed,replicas=2,resources.requests.memory=512Mi,environment.MINIO_BROWSER_REDIRECT_URL=http://localhost:30301 --create-namespace
```

## MinIO setup

In order to use the replication service it is necessary to setup manually both the requirements and the replication, either by command line or via the minIO console. We created a test environment with replication via command line as it follows.

First, we define our on minIO instances (`originminio` and `remoteminio`) on the minio client.

```bash!
mc alias set originminio https://localminio minioadminuser minioadminpassword

mc alias set remoteminio https://remoteminio minioadminuser minioadminpassword
```

A requisite for replication is enable the versioning on the buckets that will serve as origin and replica. When we create a service through OSCAR and the minIO buckets are created, versioning is not enabled by default, so we have to do it manually.

```bash!
mc version enable originminio/intermediate

mc version enable remoteminio/intermediate
```

Then, you can create the replication remote target

```bash!
mc admin bucket remote add originminio/intermediate                    \
   https://RemoteUser:Password@HOSTNAME/intermediate  \
   --service "replication"
```

and add the bucket replication rule so the actions on the origin bucket get synchronized on the replica.

```bash!
mc replicate add originminio/intermediate \
   --remote-bucket 'arn:minio:replication::<UUID>:intermediate' \
   --replicate "delete,delete-marker,existing-objects"
```

## Event handling on replication events

Once you have replica instances you can add a specific event webhook for the replica related events.

```bash!
mc event add originminio/intermediate arn:minio:sqs::intermediate:webhook --event replica
```

We observed that the replication events arrive sometimes duplicated. Although this is not yet implemented, a solution to the duplicated events would be filter them by the `userMetadata`, which is marked as *"PENDING"* on the events that we want to get rid of.

```json=
  "userMetadata": {
    "X-Amz-Replication-Status": "PENDING"
  }
```


---

***MinIO documentation used***

- [Requirements to Set Up Bucket Replication](https://min.io/docs/minio/linux/administration/bucket-replication/bucket-replication-requirements.html)
- [Enable One-Way Server-Side Bucket Replication](https://min.io/docs/minio/linux/administration/bucket-replication/enable-server-side-one-way-bucket-replication.html)