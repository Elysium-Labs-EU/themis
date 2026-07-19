Task: themis Cluster C - bare-name root exec via inherited $PATH (threat T1). External commands are run by bare name while themis runs as root so a binary planted earlier in $PATH executes as root. Fix ALL of them by resolving to absolute paths or enforcing an sbin allowlist. Extend the existing lynis hardening from commit 490d0bd to cover every external command consistently.
Branch: feat-abspath-exec-c

Work only inside /Users/rtuerlings/Coding/elysium-labs/themis/.claude/worktrees/feat-abspath-exec-c. Never delete, reset, or touch files outside it; another
agent may share the parent repo. Write a todo list before anything else.

Do the work and verify it (build + tests). Do NOT git commit or git push — argus
handles shipping. When the change is complete and tests pass, set your status
phase to "awaiting_review" (not "done"); use "blocked" if you need a decision only
the supervisor can make.

## Status reporting (required)

After each phase of your work, write your current status to
`.claude/argus/status.json` in your worktree, overwriting it each time.
Your supervisor (argus) reads this file instead of your terminal output, so keep
it accurate. Write valid JSON in exactly this shape:

    {
      "updated_at": "2026-07-18T12:00:00Z",   // RFC3339, current time
      "task": "<issue id or one-line brief>",
      "branch": "<your branch name>",
      "phase": "planning|working|self_test|awaiting_review|done|blocked",
      "real_world_proof": "<how you verified against a real target, or \"\" if n/a>",
      "pr_url": "<set once the PR exists, else \"\">",
      "blocked_reason": "<set only when phase is blocked, else \"\">",
      "files_touched": ["path/one.go", "path/two.go"],
      "tests": [
        {"cmd": "make test", "target": "./internal/...", "result": "pass|fail|skipped"}
      ],
      "diff_stat": {"files": 0, "insertions": 0, "deletions": 0}
    }

Set phase to "awaiting_review" when you want the diff reviewed, "blocked" (with
blocked_reason) when you need a decision only the supervisor can make, and "done"
only once the change is shipped. Update the file at every transition, not just at
the end.
