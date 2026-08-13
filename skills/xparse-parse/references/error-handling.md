# Error Handling — Agent Decision Guide

## Decision Matrix

Use this matrix to decide whether to STOP, RETRY, or CONFIGURE:

| Error Category | Error Codes | Decision | Action |
|---|---|---|---|
| **Transient/Network** | 30203, 500, 50207, RETRY_EXHAUSTED | STOP | CLI already exhausted its bounded retry policy; report the final error |
| **Free Limit Hit** | 40307 | STOP + ASK | Ask whether the user wants to wait for reset or explicitly use the paid API |
| **Rate Limit** | 40306 | STOP | CLI has returned its final result; report it without a Skill-layer retry |
| **File Size Exceeded** | 40302 | STOP + ASK | Automatic physical PDF splitting was unavailable or insufficient; ask before an explicit `--api paid` retry |
| **Invalid Credentials** | 40101, 40102, 40103 | STOP + DEBUG | AppKey users check TextIn console; OAuth users log in again |
| **Insufficient Balance** | 40003 | STOP + INFORM | Paid account has no credits. User must top up |
| **Unsupported File** | 40301, 40303, 40305, 40425, 40426 | STOP | File type/format not supported or corrupted. No retry |
| **Invalid Parameters** | 40004, 40400, 40424, 40427 | STOP | Command was malformed. Show correct syntax |
| **Encryption/Password** | 40422, 40423 | STOP + CLARIFY | Password required or incorrect |
| **Processing Failure** | 40428, 40429 | STOP | Office conversion failed or PDF empty. Check file integrity |
| **Unknown** | All others | STOP | Unexpected error. Show raw response for user debugging |

## Stop and Ask for Help

**When:**
- Free limit is hit (40307)
- File remains too large after the server-approved split plan (40302)
- File type is not supported (40301, 40303, 40425, 40426)
- Required user input is missing (password for encrypted doc, --page-range for large file)
- Credentials are invalid (40101, 40102, 40103)
- User's paid account has no credits (40003)
- Unknown error or internal service error

**What to say:**
- Free limit: `The free parse limit has been reached. Wait for quota reset, or explicitly approve a paid retry with --api paid. Logging in alone does not change the route.`
- File too large: `The file still exceeds a single-request limit after automatic splitting. Explicitly approve --api paid only if desired.`
- Unsupported file: `This file type is not supported. Supported formats: PDF, Word (.docx), PowerPoint (.pptx), Excel (.xlsx), OFD, Image files.`
- Password required: `Document is password-protected. Rerun with --password <your_password>.`
- Invalid credentials: `Authentication is invalid. Reconnect the WorkBuddy Connector, or check standalone AppKey credentials in the TextIn console.`

## Retry Policy

The CLI owns bounded retry and idempotency. The Skill must not invoke the same
parse again after a final error.

**Agent logic:**
```
xparse-cli parse <FILE> [options]
# Success: consume the result.
# Final failure: STOP and show the structured error and next_action.
```

**DO NOT retry when:**
- Free limit is hit (40307) — do not retry unless the user explicitly approves
  `--api paid`; logging in and rerunning the same command remains free
- File is unsupported (40301, 40303, 40425, 40426) — no retry will fix
- Credentials are invalid (40101, 40102, 40103) — user must fix credentials first
- Parameters are invalid (40004, 40400, 40424, 40427) — command syntax is wrong
- User input is missing (encrypted file without password)

**DO NOT silently skip failed parses** — always surface errors to user.

## Error Recovery Scenarios

### Scenario 1: Free Limit Hit
```
User: xparse-cli parse large-document.pdf
Error: 40307 (Free daily quota exhausted)

Agent action:
1. DO NOT retry
2. Show message:
   "The free parse limit has been reached. You can wait for quota reset.
    If you want to use the paid API, explicitly approve a retry with
    --api paid."
3. Do not treat login or stored credentials as approval to use the paid API.
4. Wait for the user's explicit choice before any paid retry.
```

### Scenario 2: Retry Exhausted
```
User: xparse-cli parse huge-file.pdf
Error: RETRY_EXHAUSTED

Agent action:
1. Do not invoke parse again.
2. Show the returned request ID, failed page range, and next action when present.
3. Ask the user to try later only when the structured error says it is safe.
```

### Scenario 3: Large File (Size Limit)
```
User: xparse-cli parse 15mb-file.pdf
Error: 40302 (File exceeds max size)

Agent action:
1. Explain that the server-selected route reached its per-request size limit;
   stored credentials did not authorize a newly chargeable route.
2. Report that the CLI could not produce physical PDF segments within the
   server limit; do not manually reproduce its split loop.
3. Ask whether the user explicitly wants a paid
   retry.
4. Only after approval, ensure paid credentials are available and retry with
   `xparse-cli parse 15mb-file.pdf --api paid`.
```

### Scenario 4: Password-Protected Document
```
User: xparse-cli parse encrypted-document.pdf
Error: 40422 (Password required)

Agent action:
1. Ask user for password
2. Rerun: xparse-cli parse encrypted-document.pdf --password <password>
3. If wrong password (40423): ask for correct password, retry once
4. If still fails: STOP, show error
```

### Scenario 5: Invalid Credentials
```
User: XPARSE_APP_ID=wrong XPARSE_SECRET_CODE=wrong xparse-cli parse doc.pdf
Error: 40102 (Invalid SECRET_CODE)

Agent action:
1. DO NOT retry
2. Show: "API credentials are invalid. 
          Check your APP_ID and SECRET_CODE in TextIn console:
          https://www.textin.com/console/dashboard/setting"
3. For standalone AppKey, direct the user to `textin-key-setup.md`.
4. For OAuth, run `xparse-cli auth device` or reconnect the WorkBuddy Connector.
```

## Integration with TextIn Setup

When a paid parse is explicitly approved and credentials are needed:

1. **WorkBuddy:** reconnect the TextIn xParse Connector; never request its
   tokens or device code in chat.
2. **Standalone OAuth:** use `xparse-cli auth device` on terminals/servers or
   `xparse-cli auth browser` on desktops.
3. **Standalone AppKey:** follow `textin-key-setup.md`.
4. Retry with an explicit `--api paid`; authentication alone never authorizes
   a newly chargeable route.

## Quick Diagnosis Flowchart

```
Parse failed?
├─ Final transient/retry error (30203, 500, 50207, RETRY_EXHAUSTED)?
│  └─ STOP; CLI already owns bounded retries
├─ Rate limit (40306)?
│  └─ STOP; report the final CLI error
├─ Free limit (40307)?
│  └─ STOP + Ask whether to wait or explicitly use --api paid
├─ File size (40302)?
│  └─ Automatic split was insufficient; ask before any --api paid retry
├─ Unsupported file (40301, 40303, 40425, 40426)?
│  └─ STOP (no retry helps)
├─ Invalid params (40004, 40400, 40424, 40427)?
│  └─ STOP + Show correct syntax
├─ Credentials invalid (40101, 40102, 40103)?
│  └─ STOP + Point to TextIn console
├─ Password issue (40422, 40423)?
│  └─ Ask user for password, retry once
├─ Processing failure (40428, 40429)?
│  └─ STOP + Check file integrity
└─ Unknown error?
   └─ STOP + Show raw error + suggest TextIn support
```

## Common Error Messages to Show Users

| Situation | Message |
|---|---|
| Free limit hit | `The free parse limit has been reached. Wait for quota reset, or explicitly approve a paid retry with --api paid. Logging in alone does not change the route.` |
| File too large | `The file remains above the per-request limit after automatic splitting. Explicitly approve --api paid only if desired.` |
| Unsupported file | `This file type is not supported. Supported: PDF, Word (.docx), PowerPoint (.pptx), Excel (.xlsx), OFD, Images.` |
| Password required | `Document is password-protected. Rerun with: xparse-cli parse <FILE> --password <your_password>` |
| Wrong password | `Password is incorrect. Try again with the correct password, or the document may use a different encryption.` |
| Invalid credentials | `API credentials are invalid. Check your APP_ID and SECRET_CODE in [TextIn console](https://www.textin.com/console/dashboard/setting).` |
| No balance | `Your paid account has insufficient balance. Purchase credits at the [TextIn xParse purchase page](https://www.textin.com/market/chager/pdf_to_markdown).` |
| Network timeout | `Service temporarily unavailable. Try again in a moment, or use --page-range to parse smaller sections.` |
