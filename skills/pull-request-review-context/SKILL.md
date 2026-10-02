---
name: pull-request-review-context
description: Extract unresolved GitHub pull request review threads and their target code lines in a compact labeled format. Use when a user asks to collect, inspect, summarize, or prepare pull request review feedback for an LLM, especially when the full diff or pull request body would add unnecessary context.
---

# Pull Request Review Context

Use `pull-request-review-context` to provide an LLM with review comments and the exact code lines they target, without fetching or repeating the full diff.

## Workflow

1. If the current checkout has an open pull request for its branch, run the command without a target. It resolves that PR and uses its base repository, including when the head branch is a fork.
2. For another pull request, pass its number or URL. For one inline review comment, pass its comment URL, such as `https://github.com/OWNER/REPOSITORY/pull/123#discussion_r456`.
3. Confirm `gh auth status` if GitHub access fails. Do not expose tokens.
4. Prefer the installed binary:

   ```bash
   pull-request-review-context
   pull-request-review-context 123
   pull-request-review-context https://github.com/OWNER/REPOSITORY/pull/123#discussion_r456
   ```

   Use `--repository OWNER/REPOSITORY` with a number when the PR is not in the current repository. Use `--all` to include resolved and outdated threads.

   If the binary is not installed and the repository is available locally, run:

   ```bash
   go run ./cmd/pull-request-review-context
   ```

5. Pass the labeled output directly as LLM context. The default includes unresolved, current review threads and excludes resolved or outdated threads. It omits comments without a recoverable target line.
6. Use `--format json` only when another program must parse the result. JSON includes comment and thread IDs, URL, path, line, body, resolved state, and outdated state. Keep the default labeled format for human or LLM review.

## Output handling

- Preserve the extracted code and review text exactly.
- Do not add the full pull request body, full diff, reviewer metadata, or unrelated files unless the user asks for them.
- Multiple comments on the same target are grouped by the CLI; retain each `review` section.
- Report an empty result as “no line-targeted review comments found,” not as an error.
- When `--all` is used, preserve the `review status` label for resolved or outdated comments.
- If the command fails, inspect current-branch PR resolution, the target URL or number, `gh` authentication, and binary availability before trying another command.
