# Configuration

## Settings

Open VSCode settings (`Ctrl+,`) and search for "craft":

### craft.server.url

**Type:** `string`
**Default:** `http://localhost:8080`

URL of the Craft diagram server.

```json
{
  "craft.server.url": "http://localhost:8080"
}
```

### craft.server.timeout

**Type:** `number`
**Default:** `30000`
**Range:** 5000-120000

Request timeout in milliseconds.

```json
{
  "craft.server.timeout": 30000
}
```

### craft.downloadPath

**Type:** `string`
**Default:** `""` (empty = prompt)

Default download directory for diagrams.

```json
{
  "craft.downloadPath": "/Users/yourname/diagrams"
}
```

### craft.logging.level

**Type:** `enum`
**Default:** `warn`
**Options:** `off`, `error`, `warn`, `info`, `debug`, `trace`

Logging verbosity level.

```json
{
  "craft.logging.level": "info"
}
```

## Complete Configuration Example

```json
{
  "craft.server.url": "http://localhost:8080",
  "craft.server.timeout": 30000,
  "craft.downloadPath": "",
  "craft.logging.level": "warn"
}
```

## Workspace Settings

You can also configure per-workspace in `.vscode/settings.json`:

```json
{
  "craft.server.url": "http://craft-server.internal:8080",
  "craft.logging.level": "debug"
}
```
