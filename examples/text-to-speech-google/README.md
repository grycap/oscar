# Text to Speech

This example applies text to speech as an OSCAR service by using the [Google Speech](https://pypi.org/project/google-speech/) library, obtaining audio files from plain text.

*Note: If you're going to invoke the service [synchronously](https://docs.oscar.grycap.net/invoking/#synchronous-invocations), you must enable a ServerlessBackend in OSCAR (Knative or OpenFaaS).*

You can specify the language of the resulting voice in the `language` environment variable of the FDL YAML file. If you don't know the code language, you can find it in this [page](https://www.andiamo.co.uk/resources/iso-language-codes/).

```yaml
functions:
  oscar:
  - oscar-cluster:
      name: text-to-speech-google
      memory: 1Gi
      cpu: '1.0'
      image: ghcr.io/grycap/text-to-speech-google
      script: script.sh
      log_level: CRITICAL
      input:
      - storage_provider: minio
        path: text-to-speech-google/input
      output:
      - storage_provider: minio
        path: text-to-speech-google/output
      environment: 
        Variables:
          language: en
```

To deploy the service use the command:
```sh
oscar-cli apply text-to-speech-google.yaml
```

To run the service synchronously use:
```sh
oscar-cli service run text-to-speech-google --text-input "Hello everyone"  --output output.mp3
```
You also can pass a file text substituing the flag `--text-input {string}` to `--input {filepath}`

And if you have installed the [VLC player](https://www.videolan.org/vlc/) and you want to play it use this one:
```sh
oscar-cli service run text-to-speech-google --text-input "Hello everyone"  --output output.mp3 && vlc output.mp3
```

You can trigger the service in an asynchronous way just uploading files to the MinIO input bucket `text-to-speech-google/input`, result files can be found in the `text-to-speech-google/output` bucket. Input and output fields in the FDL file can be removed if we are only going to use the service synchronously.
