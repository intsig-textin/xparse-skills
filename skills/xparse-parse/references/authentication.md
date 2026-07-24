# Authentication

`xparse-cli` supports three explicit authentication paths while preserving the
legacy AppKey workflow.

## WorkBuddy

WorkBuddy v5 uses the Connector's native Device Code Flow:

```bash
xparse-cli auth device --open-browser=never --output=jsonl
```

WorkBuddy extracts `verification_uri_complete` and `user_code`, presents its
Device Code Modal, opens the verification page, and leaves the CLI process
running while the CLI polls the token endpoint. The CLI saves credentials only
after authorization succeeds.

Use an isolated `XPARSE_CONFIG_DIR` for the Connector. Login and logout in
WorkBuddy must not affect standalone credentials under `~/.xparse-cli`.

The Skill must not:

- request, store, or repeat an Access Token, Refresh Token, Secret Code, or
  private `device_code`;
- implement its own callback listener, token polling, or HTTP client;
- start browser PKCE as an automatic fallback from Device OAuth;
- open a second browser when WorkBuddy already controls the Device modal.

If Connector status is disconnected, ask the user to reconnect it in
WorkBuddy. Do not collect AppKey credentials in the conversation.

## Standalone Device OAuth

Use this in terminals and on servers without a callback listener:

```bash
# Print URL/code and do not try to open a browser
xparse-cli auth device --client-id <PUBLIC_CLIENT_ID> --open-browser=never

# Open verification_uri_complete when a desktop browser is available
xparse-cli auth device --client-id <PUBLIC_CLIENT_ID> --open-browser=auto
```

`auto` and `never` run the same Device flow. Automatic browser opening is only a
convenience; authorization state remains on the server and the CLI continues
polling.

Client ID resolution is:

1. `--client-id`
2. `XPARSE_OAUTH_CLIENT_ID`
3. `config.yaml` `oauth.client_id`
4. the shipped public xParse client `cli_textin_xparse`

The default client ID is public application identity, not a client secret. A
private deployment may override it with any of the first three options.

Scope resolution is flag, `XPARSE_OAUTH_SCOPE`, YAML, then `ocr:*`.

## Standalone Browser PKCE

Use browser PKCE only as an explicit desktop choice:

```bash
xparse-cli auth browser --client-id <PUBLIC_CLIENT_ID>
```

It opens Authorization Code + PKCE and listens only on the configured
`http://127.0.0.1` loopback callback. It is not an implicit fallback from
Device OAuth.

## Standalone AppKey

The legacy command remains compatible:

```bash
xparse-cli auth
# Equivalent explicit command:
xparse-cli auth app-key
```

For automation, callers may provide a complete pair through
`XPARSE_APP_ID` and `XPARSE_SECRET_CODE`. Never print or copy the Secret into
agent output.

## Status and logout

```bash
xparse-cli auth status --output=json
xparse-cli auth logout --method oauth
xparse-cli auth logout --method app-key
xparse-cli auth logout --method all
```

Status is local and does not make a network request. An expired access token
with a still-usable refresh token remains logged in; the next paid OAuth parse
refreshes it.
