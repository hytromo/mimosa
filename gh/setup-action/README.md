# hytromo/mimosa/gh/setup-action

See [here](../../docs/gh-actions/README.md) for example usage.

## Inputs
| Name | Description | Required | Default |
| --- | --- | --- | --- |
| `version` | The mimosa binary version | false | `latest` |
| `tools-file` | The .tool-versions asdf/mise file to extract the mimosa version from | false | `.tool-versions` |

## Outputs
| Name | Description |
| --- | --- |
| `binary-path` | The full path to the mimosa binary |
| `cache-path` | The full path to the mimosa cache directory |