# Managed Volume Demo

This example shows how to use the `volume` field to create a managed volume with a name derived from the service name and persist data across invocations.

## Files

- `volume-demo.yaml`: service definition with a managed volume.
- `script.sh`: increments a counter stored in `/data/counter.txt`.

## Deploy

```bash
oscar service deploy examples/volumes/volume-demo.yaml
```

Expected behavior:
- OSCAR creates a managed volume because `volume.size` is set.
- The logical volume name is derived from the service name `volume-demo`.
- The volume is mounted at `/data`.

## Invoke multiple times

```bash
curl -X POST "$OSCAR_ENDPOINT/run/volume-demo" -u "$OSCAR_USER:$OSCAR_PASS"
curl -X POST "$OSCAR_ENDPOINT/run/volume-demo" -u "$OSCAR_USER:$OSCAR_PASS"
curl -X POST "$OSCAR_ENDPOINT/run/volume-demo" -u "$OSCAR_USER:$OSCAR_PASS"
```

The returned `counter` should increase (`1`, `2`, `3`...), proving data is persisted in the volume.

You can inspect the created volume with:

```bash
curl -X GET "$OSCAR_ENDPOINT/system/volumes/volume-demo" -u "$OSCAR_USER:$OSCAR_PASS"
```

## Cleanup

```bash
oscar service remove volume-demo
```

With the default lifecycle, deleting the service also deletes its managed volume.
