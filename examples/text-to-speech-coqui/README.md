# Text to Speech

This example applies text to speech as an OSCAR service by using the [coqui-ai text to speech](https://github.com/coqui-ai/TTS) library, obtaining audio files from plain text.

*Note: If you're going to invoke the service [synchronously](https://docs.oscar.grycap.net/invoking/#synchronous-invocations), you must enable a ServerlessBackend in OSCAR (Knative or OpenFaaS).*

```yaml
functions:
  oscar:
  - oscar-cluster:
      name: text-to-speech-coqui
      memory: 2Gi
      cpu: '4.0'
      image: ghcr.io/grycap/text-to-speech-coqui
      log_level: CRITICAL
      script: script.sh
      input:
      - storage_provider: minio
        path: text-to-speech-coqui/input
      output:
      - storage_provider: minio
        path: text-to-speech-coqui/output
```

To deploy the service use the command:

```sh
oscar-cli apply text-to-speech-coqui.yaml
```

To run the service synchronously use:

```sh
oscar-cli service run text-to-speech-coqui --text-input "Hello everyone"  --output output.mp3
```

You also can pass a file text substituing the flag `--text-input {string}` to `--input {filepath}`

And if you have installed the [VLC player](https://www.videolan.org/vlc/) and you want to play it use this one:

```sh
oscar-cli service run text-to-speech-coqui --text-input "Hello everyone"  --output output.mp3 && vlc output.mp3
```

You can trigger the service in an asynchronous way just uploading files to the MinIO input bucket `text-to-speech-coqui/input`, result files can be found in the `text-to-speech-coqui/output` bucket. Input and output fields in the FDL file can be removed if we are only going to use the service synchronously.
