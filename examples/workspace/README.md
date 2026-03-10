# Workspace Demo

This example shows how to use the new `workspace` field to persist data across invocations.

## Files

- `workspace-demo.yaml`: service definition with managed workspace.
- `script.sh`: increments a counter stored in `/data/counter.txt`.

## Deploy

```bash
oscar service deploy examples/workspace/workspace-demo.yaml
```

## Invoke multiple times

```bash
curl -X POST "$OSCAR_ENDPOINT/run/workspace-demo" -u "$OSCAR_USER:$OSCAR_PASS"
curl -X POST "$OSCAR_ENDPOINT/run/workspace-demo" -u "$OSCAR_USER:$OSCAR_PASS"
curl -X POST "$OSCAR_ENDPOINT/run/workspace-demo" -u "$OSCAR_USER:$OSCAR_PASS"
```

The returned `counter` should increase (`1`, `2`, `3`...), proving data is persisted in the workspace.

## Cleanup

```bash
oscar service remove workspace-demo
```

With the default lifecycle, deleting the service also deletes its workspace.
