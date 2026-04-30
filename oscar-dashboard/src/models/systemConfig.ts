import { MinioStorageProvider } from "@/pages/ui/services/models/service";

export type SystemConfig = {
  minio_provider: MinioStorageProvider;
  name: string;
  namespace: string;
  gpu_available: boolean;
  serverless_backend: string;
  yunikorn_enable: boolean;
  interLink_available: boolean;
  oidc_groups: string[];
};
