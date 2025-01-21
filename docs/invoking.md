# Service Execution Types

OSCAR services can be executed:

  - [Synchronously](invoking-sync.md), so that the invocation to the service blocks the client until the response is obtained. 
  - [Asynchronously](invoking-async.md), typically in response to a file upload to MinIO or via the OSCAR API.
  - As an [exposed service](exposed-services.md), where the application executed already provides its own API or user interface (e.g. Jupyter Notebook)


After reading the different service execution types, take into account the following considerations to better decide the most appropriate execution type for your use case:

* **Scalability**: Asynchronous invocations provide the best throughput when dealing with multiple concurrent data processing requests, since these are processed by independent jobs which are managed by the Kubernetes scheduler. A two-level elasticity approach is used (increase in the number of pods and increase in the number of Virtual Machines, if the OSCAR cluster was configured to be elastic). This is the recommended approach when each processing request exceeds the order of tens of seconds.

* **Reduced Latency** Synchronous invocations are oriented for short-lived (< tens of seconds) bursty requests. A certain number of containers can be configured to be kept alive to avoid the performance penalty of spawning new ones while providing an upper bound limit (see [`min_scale` and `max_scale` in the FDL](fdl.md#synchronoussettings), at the expense of always consuming resources in the OSCAR cluster. If the processing file is in the order of several MBytes it may not fit in the payload of the HTTP request.

* **Easy Access** For services that provide their own user interface or their own API, exposed services provide the ability to execute them in OSCAR and benefit for an auto-scaled configuration in case they are [stateless](https://en.wikipedia.org/wiki/Service_statelessness_principle). This way, users can directly access the service using its well-known interfaces by the users. 

If you want to process a large number of files, please consider using [OSCAR-batch](https://github.com/grycap/oscar-batch).

