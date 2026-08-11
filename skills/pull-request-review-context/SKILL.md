---
name: pull-request-review-context
description: Extract GitHub pull request review comments together with only their target code lines in a compact labeled format. Use when a user asks to collect, inspect, summarize, or prepare pull request review feedback for an LLM, especially when the full diff or pull request body would add unnecessary context.
---

# Pull Request Review Context

Use `pull-request-review-context` to provide an LLM with review comments and the exact code lines they target, without fetching or repeating the full diff.

## Workflow

1. Resolve the repository and pull request number. Accept `OWNER/REPOSITORY NUMBER` or a pull request URL.
2. Confirm `gh auth status` if GitHub access fails. Do not expose tokens.
3. Prefer the installed binary:

   ```bash
   pull-request-review-context --repository OWNER/REPOSITORY NUMBER
   ```

   If it is not installed and the repository is available locally, run:

   ```bash
   go run ./cmd/pull-request-review-context --repository OWNER/REPOSITORY NUMBER
   ```

4. Pass the labeled output directly as LLM context. The default output contains `file`, `line`, `code`, and `review` sections. It excludes file-level comments and comments without a target line.
5. Use `--format json` only when another program must parse the result. Keep the default labeled format for human or LLM review.

## Output handling

- Preserve the extracted code and review text exactly.
- Do not add the full pull request body, full diff, reviewer metadata, or unrelated files unless the user asks for them.
- Multiple comments on the same target are grouped by the CLI; retain each `review` section.
- Report an empty result as “no line-targeted review comments found,” not as an error.
- If the command fails, inspect repository resolution, pull request number, `gh` authentication, and binary availability before trying another command.
