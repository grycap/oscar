# Service Execution Types

OSCAR services can be executed:

  - [Synchronously](invoking-sync.md), so that the invocation to the service blocks the client until the response is obtained. Useful for short-lived service invocations.
  - [Asynchronously](invoking-async.md), typically in response to a file upload to MinIO or via the OSCAR API.
  - As an [exposed service](exposed-services.md), where the application executed already provides its own API or user interface (e.g. a Jupyter Notebook)

