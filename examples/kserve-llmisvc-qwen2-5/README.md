# KServe LLM: Qwen2.5-0.5B-Instruct (vLLM CPU)

This example deploys an LLM service on OSCAR using KServe,
vLLM on CPU, and an OCI modelcar image that contains the
`Qwen/Qwen2.5-0.5B-Instruct` model.

## Example files

| File | Description |
|---|---|
| `fdl.yaml` | OSCAR service definition with a KServe `llm_inference` block. |
| `docker/Dockerfile.vllm` | vLLM CPU runtime wrapper with user `uid=1010` for KServe modelcar compatibility. |
| `docker/Dockerfile.model` | Modelcar image that downloads the model from Hugging Face. |

## Requirements

- OSCAR cluster with KServe enabled.
- `oscar-cli` configured against your cluster.

## 1. Deploy the service

```bash
oscar-cli apply fdl.yaml
```

Verify that the service was created:

```bash
oscar-cli service list
```

The service name in this example is `qwen2-5-05b-instruct`.

## 2. Test the OpenAI-compatible endpoint

Once the service is ready, the model will be exposed on `https://<YOUR_CLUSTER>/system/services/<SERVICE_NAME>/models` and you can test your service in different ways:

### Direct request with `curl`

1. Open a terminal and try:

    ```bash
    curl -X POST "https://<YOUR_CLUSTER>/system/services/qwen2-5-05b-instruct/models/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer <TOKEN>" \
        --data '{
            "model": "qwen2-5-05b-instruct",
            "messages": [
                {
                    "role": "user",
                    "content": "Write a short explanation about KServe"
                }
            ]
        }'
    ```
    > Replace `<TOKEN>` with your service token or four personal OIDC token.  
    
    > Note: If there is only one model, it will have the same name as the OSCAR service.

### Through Open WebUI

1. Install [Docker](https://www.docker.com)
2. Run Open WebUI:
    ```bash
    docker run -d -p 3000:8080 -e WEBUI_AUTH=False -v open-webui:/app/backend/data --name open-webui ghcr.io/open-webui/open-webui:main
    ```
3. Go to [http://localhost:3000/](http://localhost:3000/)
4. Add a connection to the service:  
    `Top right corner → Admin Panel → Settings → Connections → OpenAI API`
5. Try it

## Build the images

### vLLM CPU runtime

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/grycap/kserve-vllm-openai-cpu:v0.22.1 -f Dockerfile.vllm . --push
```

### OCI modelcar (Qwen2.5 model)

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/grycap/kserve-qwen2-5-05b-instruct:latest -f Dockerfile.model . --push
```

If you use a local registry (for example `localhost:5001`), update the tags in
the commands above and in `fdl.yaml` (`runtime_image` and `storage_uri`).

## Notes

- The first startup can take several minutes (model download and pod rollout).
- The current example defines modest resources (`cpu: 2`, `memory: 6Gi`); adjust them for your cluster.
- `fdl.yaml` uses `--dtype=auto` and `--enforce-eager` for more stable CPU execution.

## Additional Resources

- [vLLM Documentation](https://docs.vllm.ai/en/latest/)
- [OSCAR Documentation](https://docs.oscar.grycap.net/)
- [KServe](https://kserve.github.io/website/)
- [API](https://docs.oscar.grycap.net/latest/api/)
- [OSCAR CLI](https://github.com/grycap/oscar-cli)