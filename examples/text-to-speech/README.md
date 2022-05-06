# Text to Speech

Example of text to speech in OSCAR using [Google Speech](https://pypi.org/project/google-speech/) library, where introducing an input of text string or text file return an audio file.

Note: If you invoke synchronously, you must enable a ServerlessBackend (Knative or OpenFaaS).

In the yaml you will select in which language you want to hear the voice by changing the language variable. If you do not know the code language, you will found it in this [page](https://www.andiamo.co.uk/resources/iso-language-codes/).

```yaml
functions:
  oscar:
  - oscar-cluster:
      name: text-to-speech
      memory: 1Gi
      cpu: '1.0'
      image: ghcr.io/grycap/text-to-speech
      script: script.sh
      log_level: CRITICAL
      input:
      - storage_provider: minio
        path: text-to-speech/input
      output:
      - storage_provider: minio
        path: text-to-speech/output
      environment: 
        Variables:
          language: en
```

To deploy the service use the command:
```sh
oscar-cli apply tts.yaml
```

To run the service synchronously use:
```sh
oscar-cli service run text-to-speech --text-input "Hello everyone"  --output "output.mp3"
```
You also can pass a file text substituing the flag `--text-input {string}` to `--input {filepath}`

And if you have installed vlc and you want to play it use this one:
```sh
oscar-cli service run text-to-speech --text-input "Hello everyone"  --output "output.mp3" && vlc output.mp3
```

You can trigger the service in an asynchronous way just uploading a file to a minio bucket in `text-to-speech/input` and the result can be found in `text-to-speech/output`. Input and output fields in yaml file could be remove if only we are going to use the services synchronously.
