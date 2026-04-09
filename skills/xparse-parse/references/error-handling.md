# Error Handling — Agent Decision Guide

## Decision Matrix

Use this matrix to decide whether to STOP, RETRY, or CONFIGURE:

| Error Category | Error Codes | Decision | Action |
|---|---|---|---|
| **Transient/Network** | 30203, 500, 50207 | RETRY (once) | Retry same command with backoff |
| **Free Limit Hit** | 40307 | STOP + CONFIGURE | Show free-limit message, point to setup guide |
| **Rate Limit** | 40306 | RETRY (with delay) | Reduce request frequency, retry later |
| **File Size Exceeded** | 40302 | STOP + CONFIGURE or ADJUST | If ≤10MB: likely free limit → configure. If >10MB: suggest `--page-range` or paid API |
| **Invalid Credentials** | 40101, 40102, 40103 | STOP + DEBUG | User's credentials are wrong. Check TextIn console |
| **Insufficient Balance** | 40003 | STOP + INFORM | Paid account has no credits. User must top up |
| **Unsupported File** | 40301, 40303, 40305, 40425, 40426 | STOP | File type/format not supported or corrupted. No retry |
| **Invalid Parameters** | 40004, 40400, 40424, 40427 | STOP | Command was malformed. Show correct syntax |
| **Encryption/Password** | 40422, 40423 | STOP + CLARIFY | Password required or incorrect |
| **Processing Failure** | 40428, 40429 | STOP | Office conversion failed or PDF empty. Check file integrity |
| **Unknown** | All others | STOP | Unexpected error. Show raw response for user debugging |

## Stop and Ask for Help

**When:**
- Free limit is hit (40307)
- File is too large for free tier (40302) and no page range applied
- File type is not supported (40301, 40303, 40425, 40426)
- Required user input is missing (password for encrypted doc, --page-range for large file)
- Credentials are invalid (40101, 40102, 40103)
- User's paid account has no credits (40003)
- Unknown error or internal service error

**What to say:**
- Free limit: `The free parse limit has been reached. Configure your TextIn API credentials with your APP_ID and SECRET_CODE, then rerun the same parse command. See [TextIn setup guide] for details.`
- File too large: `File exceeds 10MB free tier limit. Use --page-range to parse specific pages, or configure paid API credentials for unlimited file sizes.`
- Unsupported file: `This file type is not supported. Supported formats: PDF, Word (.docx), PowerPoint (.pptx), Excel (.xlsx), OFD, Image files.`
- Password required: `Document is password-protected. Rerun with --password <your_password>.`
- Invalid credentials: `API credentials are invalid. Check your APP_ID and SECRET_CODE in the TextIn console at https://www.textin.com/console/dashboard/setting.`

## Retry Policy

**Retry exactly once when:**
- Base service fault (30203)
- Internal server error (500)
- Partial parse failure (50207)

**Retry logic:**
```
xparse-cli parse <FILE> [options]
# If fails with 50xxx (transient):
xparse-cli parse <FILE> [options]  # Retry once, same command
# If fails again or different error: STOP and show user
```

**DO NOT retry when:**
- Free limit is hit (40307) — user must configure API
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
   "The free parse limit has been reached. Configure your TextIn API 
    credentials, then rerun the same parse command. 
    See [TextIn setup guide] for details."
3. Point user to ~/xparse-parse/references/textin-key-setup.md
4. Wait for user to configure credentials before retrying
```

### Scenario 2: File Too Large (Transient Network)
```
User: xparse-cli parse huge-file.pdf
Error: 30203 (Base service fault)

Agent action:
1. Retry once: xparse-cli parse huge-file.pdf
2. If succeeds: done
3. If fails again: STOP
   Show: "Service is temporarily unavailable. Try again in a few moments, 
          or use --page-range to parse smaller sections."
```

### Scenario 3: Large File (Size Limit)
```
User: xparse-cli parse 15mb-file.pdf
Error: 40302 (File exceeds max size)

Agent action:
IF user has NOT configured API credentials:
  "File exceeds 10MB free tier limit. 
   Option 1: Use --page-range 1-5 to parse first few pages
   Option 2: Configure paid API for unlimited file sizes (setup guide)"
   
IF user HAS configured credentials:
  "File size error. Credentials may be invalid or account has no balance.
   Try: xparse-cli parse 15mb-file.pdf --page-range 1-5
   Or check credentials at TextIn console."
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
3. Suggest user re-run setup: bash ~/xparse-parse/setup.sh
```

## Integration with TextIn Setup

When free limit is hit or credentials needed:

1. **Point to setup guide:** `~/xparse-parse/references/textin-key-setup.md`
2. **User gets credentials** from https://www.textin.com/console/dashboard/setting
3. **User exports env vars** (setup.sh can help)
4. **Rerun same parse command** — no need to change anything else
5. **All limits removed** — file size, daily quota

If user is uncertain about setup:
```
Run: bash ~/xparse-parse/setup.sh

This will help you:
1. Get credentials from TextIn console
2. Export XPARSE_APP_ID and XPARSE_SECRET_CODE
3. Verify setup is working
```

## Quick Diagnosis Flowchart

```
Parse failed?
├─ Transient (30203, 500, 50207)?
│  └─ Retry once, then STOP if still fails
├─ Rate limit (40306)?
│  └─ Wait and retry with reduced frequency
├─ Free limit (40307)?
│  └─ STOP + Point to TextIn setup guide
├─ File size (40302)?
│  └─ Have credentials? → might be account issue
│     No credentials? → suggest --page-range or setup guide
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
| Free limit hit | `The free parse limit has been reached. Configure your TextIn API credentials, then rerun the same parse command. See [TextIn setup guide](textin-key-setup.md) for details.` |
| File too large (no creds) | `File exceeds 10MB free tier limit. Use --page-range 1-5 to parse specific pages, or [configure paid API](textin-key-setup.md) for unlimited sizes.` |
| Unsupported file | `This file type is not supported. Supported: PDF, Word (.docx), PowerPoint (.pptx), Excel (.xlsx), OFD, Images.` |
| Password required | `Document is password-protected. Rerun with: xparse-cli parse <FILE> --password <your_password>` |
| Wrong password | `Password is incorrect. Try again with the correct password, or the document may use a different encryption.` |
| Invalid credentials | `API credentials are invalid. Check your APP_ID and SECRET_CODE in [TextIn console](https://www.textin.com/console/dashboard/setting).` |
| No balance | `Your paid account has insufficient balance. Top up credits at [TextIn console](https://www.textin.com/console/dashboard/setting).` |
| Network timeout | `Service temporarily unavailable. Try again in a moment, or use --page-range to parse smaller sections.` |
