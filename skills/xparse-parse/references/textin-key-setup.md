# TextIn Key Setup

Configure paid API credentials to remove file size limits and increase daily quota.

## When to Configure

- Free daily limit exceeded (40001 error)
- File size exceeds 10MB limit (40302 error)
- Want unlimited quota for production use

## Setup Steps

### Option 1: Interactive Setup (recommended)

```bash
xparse-cli auth                    # Interactive credential setup
```

Follow the prompts to enter your `APP_ID` and `SECRET_CODE` from [TextIn Console](https://www.textin.com/console/dashboard/setting). Credentials are saved to `~/.xparse-cli/config.yaml`.

### Option 2: Environment Variables

For CI/automation, set environment variables:

```bash
export XPARSE_APP_ID=<your_app_id>
export XPARSE_SECRET_CODE=<your_secret_code>
```

### Verify Setup

```bash
xparse-cli auth --show             # Show current credential source
xparse-cli parse <FILE>            # Should succeed without "unauthorized" errors
```

Credential priority: CLI flags → env vars → `~/.xparse-cli/config.yaml`

## Troubleshooting

For all error codes and recovery actions, see [error-handling.md](error-handling.md).

## References

- [TextIn Console](https://www.textin.com/console/dashboard/setting)
- [TextIn Documentation](https://docs.textin.com/)
