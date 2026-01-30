# Data Model: Federated OSCAR Service Replicas

## Entities

### Service
- **Description**: OSCAR service definition managed by OSCAR Manager.
- **Key fields**: name, image, script, resources, input[], output[], federation.
- **Relationships**:
  - One Service can belong to one Federation.
  - One Service can have many Replicas (references to other services).

### Federation
- **Description**: Logical group of services participating in a federation.
- **Key fields**:
  - `group_id` (string, unique within a federation scope)
  - `topology` (enum: none | tree | mesh)
  - `delegation` (enum: static | random | load-based)
  - `members` (list of ReplicaRef)
- **Relationships**:
  - Federation groups multiple Services.

### ReplicaRef
- **Description**: Reference to a service replica in another cluster.
- **Key fields**: type (e.g., oscar), cluster_id, service_name, priority.
- **Constraints**:
  - `cluster_id` + `service_name` uniquely identify a replica target.

### DelegationPolicy
- **Description**: Policy for selecting target replicas.
- **Key fields**: mode (static | random | load-based), weights (optional).
- **Relationships**: Applied to Federation.

### ClusterStatus
- **Description**: Snapshot of cluster capacity/health used for delegation.
- **Key fields**:
  - cpu.total_free, cpu.max_free_on_node
  - memory.total_free_bytes, memory.max_free_on_node_bytes
  - nodes[] (name, cpu, memory, has_gpu, status)
- **Relationships**: Queried across federation members.

## Validation Rules
- `group_id` defaults to the service name if omitted.
- `topology` MUST be one of none/tree/mesh.
- Federation expansion occurs only when `federation.members` is non-empty.
- Replica updates MUST apply to the whole topology.
- Delegation MUST only target clusters where inputs are accessible.

## State/Transitions (conceptual)
- Service lifecycle: create → update → delete (existing OSCAR flow).
- Federation lifecycle: create (via coordinator FDL) → expand → maintain via
  /system/replicas → delete.
