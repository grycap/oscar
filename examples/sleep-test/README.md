Simple OSCAR example that sleeps for a configurable amount of time.

## Overview

The service runs a tiny script that reads the environment variable `SLEEP_SECONDS` (default: `30`) and sleeps for that many seconds, printing a message before and after the sleep.

## Files

- [`sleep-test.yaml`](sleep-test.yaml): FDL for the service.
- [`script.sh`](script.sh): Bash script executed by the service.

## Deploy

```sh
oscar-cli apply sleep-test.yaml
```

## Run (sync example)

```sh
oscar-cli service run sleep-test --env SLEEP_SECONDS=15
```

Output:
```
Sleeping for 15s...
Finished sleeping for 15s.
```

## Run (async example)

Submit without extra env to use the default 30 seconds:

```sh
oscar-cli service job sleep-test --text-input "Sleep Test"
```

Check logs for the start/finish messages once the job completes.
