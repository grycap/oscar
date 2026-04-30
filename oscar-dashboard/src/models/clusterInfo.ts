export type ClusterInfo = {
  version: string;
  git_commit: string;
  architecture: string;
  kubernetes_version: string;
  name: string;
  serverless_backend: {
    name:string;
    version:string;
  };
};