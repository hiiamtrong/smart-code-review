---
phase: implementation
title: Implementation - Toggle AI Review
description: Implementation notes for AI review toggle
---

# Implementation - Toggle AI Review

## Files to Modify

| File | Change |
|------|--------|
| `scripts/local/ai-review` | Add prompt in `cmd_setup()`, add to help text |
| `scripts/local/pre-commit.sh` | Add default in `load_config()`, add toggle check |
| `SETUP_GUIDE.md` | Document new config option |
| `README.md` | Update configuration section |

## Implementation Pattern

Follow the `ENABLE_SONARQUBE_LOCAL` pattern:

```bash
# In load_config():
ENABLE_AI_REVIEW="${ENABLE_AI_REVIEW:-true}"

# In main flow:
if [[ "$ENABLE_AI_REVIEW" != "true" ]]; then
  log_info "AI Review disabled (enable: ai-review config set ENABLE_AI_REVIEW true)"
  # skip AI review, continue to results
fi
```

## Key Considerations

- Credentials (`AI_GATEWAY_URL`, `AI_GATEWAY_API_KEY`) should still be collected during setup even if AI review is disabled, so re-enabling is seamless
- When both AI review and SonarQube are disabled, the hook should exit early with an info message
