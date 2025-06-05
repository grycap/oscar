# Node-RED OSCAR Example

This example demonstrates how to deploy [Node-RED](https://nodered.org/) as a service on an OSCAR cluster using an FDL definition and a custom Docker image. The setup includes preloading OSCAR subflows and configuring persistent storage.

## Contents

- `Dockerfile`: Builds a Node-RED image with OSCAR nodes and [subflows](https://github.com/ai4os/ai4-compose/tree/main/node-red/flows).
- `fdl.yaml`: FDL definition for deploying Node-RED.
- `script.sh`: Startup script for initializing directories and launching Node-RED.
- `with_auth/`: Example with admin authentication enabled.

## Deployment

### 1. Build and Push the Docker Image

If you want to use your own image, build and push it to a registry:

```bash
docker build -t <your-registry>/node-red-oscar:latest .
docker push <your-registry>/node-red-oscar:latest
```

Update the `image:` field in `fdl.yaml` if you use a custom image.

### 2. Deploy with OSCAR CLI

Apply the FDL file to your OSCAR cluster:

```bash
oscar-cli apply fdl.yaml
```

For the version with authentication, use the files in the `with_auth/` directory:

```bash
cd with_auth
oscar-cli apply fdl.yaml
```

### 3. Access Node-RED

Once deployed, access Node-RED at:

```
https://<YOUR_CLUSTER>/system/services/node-red/exposed/
```

For the authenticated version, log in with:

- **Username:** `admin`
- **Password:** as set in the `PASSWORD` secret (default: `admin`)

## Customization

- To add or modify preinstalled nodes or subflows adjust the `Dockerfile`.
- To customize Node-Red deployment options see `script.sh`.

## References

- [Node-RED Documentation](https://nodered.org/docs/)
- [OSCAR Documentation](https://docs.oscar.grycap.net/)
- [OSCAR CLI](https://github.com/grycap/oscar-cli)
- [Node-Red: AI4Compose collection](https://flows.nodered.org/collection/vAqHyycWgCq_)

---