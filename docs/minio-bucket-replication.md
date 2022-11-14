# MinIO bucket replication

In scenarios where you have two linked OSCAR clusters as part of the same workflow defined in [FDL](https://docs.oscar.grycap.net/fdl/), temporary network disconnections cause that data generated on the first cluster during the disconnection time is lost as well. 

To resolve this scenario we propose the use of replicated buckets on MinIO. With this approach, you can have two buckets synchronized on different OSCAR clusters so that, if the connection is lost, they will be re-synchronized when the connection is restored.

An example of this scenario is shown on the following diagram, where there are two MinIO instances (each one on a different OSCAR cluster), and the output of the execution of *service_x* on the source serves as input for the *service_y* on the remote cluster.

![minio-replication-diagram](images/minio-bucket-replication/minio-replication-diagram.png)

Here is in more detail the data flow between the buckets:

**MinIO instance source**
- `input`: receives data and triggers the execution of OSCAR *service_x*.
- `intermediate`: the output files from *service_x* are stored on this bucket and synchronized with the intermediate bucket on the remote instance. 

**MinIO instance remote**
- `intermediate`: the synchronized bucket that stores the replicated data and triggers OSCAR *service_y*.
- `output`: stores the output files of *service_y*.

### Considerations

When you create the service on the remote OSCAR cluster, the `intermediate` bucket which is both the replica and input of the OSCAR service will have the webhook event for PUT actions enabled so it can trigger the OSCAR service.

Because, as explained below on [Event handling on replication events](#Event-handling-on-replication-events), there are some specific events for replicated buckets, it is important to delete this event webhook to avoid getting both events every time.

```
mc event remove originminio/intermediate arn:aws:sqs::intermediate:webhook --event put
```

## Helm installation

To be able to use replication each MinIO instance deployed with Helm has to be configured in distributed mode. This is done by adding the parameters `mode=distributed,replicas=NUM_REPLICAS`.

Here is an example of a local MinIO replicated deployment with Helm:

```
helm install minio minio/minio --namespace minio --set rootUser=minio,rootPassword=minio123,service.type=NodePort,service.nodePort=30300,consoleService.type=NodePort,consoleService.nodePort=30301,mode=distributed,replicas=2,resources.requests.memory=512Mi,environment.MINIO_BROWSER_REDIRECT_URL=http://localhost:30301 --create-namespace
```

## MinIO setup

To use the replication service it is necessary to set up manually both the requirements and the replication, either by command line or via the MinIO console. We created a test environment with replication via the command line as follows.

First, we define our minIO instances (`originminio` and `remoteminio`) on the minio client.

```
mc alias set originminio https://localminio minioadminuser minioadminpassword

mc alias set remoteminio https://remoteminio minioadminuser minioadminpassword
```

A requisite for replication is to enable the versioning on the buckets that will serve as origin and replica. When we create a service through OSCAR and the minIO buckets are created, versioning is not enabled by default, so we have to do it manually.

```
mc version enable originminio/intermediate

mc version enable remoteminio/intermediate
```

Then, you can create the replication remote target

```
mc admin bucket remote add originminio/intermediate \
  https://RemoteUser:Password@HOSTNAME/intermediate \
  --service "replication"
```

and add the bucket replication rule so the actions on the origin bucket get synchronized on the replica.

```
mc replicate add originminio/intermediate \
   --remote-bucket 'arn:minio:replication::<UUID>:intermediate' \
   --replicate "delete,delete-marker,existing-objects"
```

## Event handling on replication events

Once you have replica instances you can add a specific event webhook for the replica-related events.

```
mc event add originminio/intermediate arn:minio:sqs::intermediate:webhook --event replica
```

The replication events sometimes arrive duplicated. Although this is not yet implemented, a solution to the duplicated events would be to filter them by the `userMetadata`, which is marked as *"PENDING"* on the events to be discarded.

```
  "userMetadata": {
    "X-Amz-Replication-Status": "PENDING"
  }
```


---

***MinIO documentation used***

- [Requirements to Set Up Bucket Replication](https://min.io/docs/minio/linux/administration/bucket-replication/bucket-replication-requirements.html)
- [Enable One-Way Server-Side Bucket Replication](https://min.io/docs/minio/linux/administration/bucket-replication/enable-server-side-one-way-bucket-replication.html)