import { Service, LOG_LEVEL } from "../../../models/service";

export const defaultService: Service = {
  name: "",
  cluster_id: "",
  memory: "256Mi",
  cpu: "0.2",
  enable_gpu: false,
  total_memory: "0",
  total_cpu: "0",
  synchronous: {
    min_scale: 0,
    max_scale: 0,
  },
  replicas: [],
  rescheduler_threshold: "",
  token: "",
  log_level: LOG_LEVEL.INFO,
  image_rules: [],
  image: "",
  alpine: false,
  script: "",
  image_pull_secrets: [],
  environment: {
    variables: {},
    secrets: {},
  },
  annotations: {},
  labels: {},
  input: [],
  output: [],
  owner: "",
  storage_providers: {
    s3: undefined,
    minio: undefined,
    onedata: undefined,
    webdav: undefined,
  },
  clusters: {
    id: {
      endpoint: "",
      auth_user: "",
      auth_password: "",
      ssl_verify: false,
    },
  },
  interlink_node_name: "",
  allowed_users: [],
  expose: {
    min_scale: "",
    max_scale: "",
    api_port: "",
    cpu_threshold: "",
    rewrite_target: false,
    nodePort: "",
    default_command: false,
    set_auth: false
  }
};
