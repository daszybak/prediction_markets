# Claude Code Orchestration & Cron — Field Research

Survey of 15 cloned repos plus the broader ecosystem of facilitating projects.
Captured 2026-05-17. Snapshots are `git clone --depth=1` so commit hashes aren't
quoted; line numbers refer to the head of `main`/`master` at the time of
research.

## TL;DR

- **Orchestration** has standardized around a few primitives: SendMessage-style
  agent pipelines, git-worktree fan-out, soft-enforcement nudges instead of hard
  blocks, YAML/JSON handoff files surviving compaction, and per-agent isolated
  contexts.
- **Cron/scheduled** is the ecosystem's biggest gap. The community has answered
  with three pattern families: (1) external schedulers calling `claude -p`
  headless, (2) MCP-server-as-cron, (3) GitHub Actions `schedule:` triggers.
- The most novel single project found: **rohitg00/pro-workflow**'s cron-tick
  driver + SQLite seed queue + STOP file kill-switch — a minimal recipe that
  works on any host with cron.

---

## Part 1 — Coolest implementations from the 15 cloned repos

### 1. ruvnet/ruflo — SendMessage pipelines
The orchestration model worth stealing: named agents push results forward via
`SendMessage` instead of pulling from shared state. Three topologies
(hierarchical / mesh / adaptive) reuse the same primitive.

```
architect → SendMessage → developer → SendMessage → tester → SendMessage → reviewer
```

- `CLAUDE.md:544-606` — pipeline definition
- `CLAUDE.md:68-102` — MCP swarm-init + Task spawn in the **same message** to
  avoid sequential bottlenecks
- `daemon start` runs 12 background workers (ultralearn, optimize, audit…)
- Recent PR signal: #2031 wires MCP through the real ConsensusEngine; #2023
  adds a custom-worker manifest schema — the project is actively investing in
  pluggable worker registration.

### 2. wshobson/agents — PluginEval 3-layer quality gate
Pre-deployment grading for any agent/skill:

1. Deterministic structural scan (<2s, free)
2. LLM semantic judge — 4 Claude calls, ~30s
3. Monte Carlo reliability — 50–100 simulated runs, 2–5min

Weighted on 10 dimensions; `triggering_accuracy` (25%) and
`orchestration_fitness` (20%) dominate. `CLAUDE.md:90-127`. PR #535 ("agent
teams coordination guardrails") is the active edge.

### 3. barkain/claude-code-workflow-orchestration — soft enforcement
Best example of *not* hard-blocking the agent. Per-turn counter on 7 work
primitives escalates silent → "STOP" → strong reminder; resets on
`/workflow-orchestrator:delegate`; bypassed when running as a subagent (env
`CLAUDE_PARENT_SESSION_ID`). `CLAUDE.md:110-129`.

Even cooler: the **plan-mode sandwich with recovery**. State is persisted to
`.claude/state/approved_execution_plan.json` *before* `ExitPlanMode` so the
plan survives context compaction. PR #42 explicitly fixed the
delegate→plan→approve infinite loop; #47 made the recovery durable after
context clears. `commands/delegate.md:7-11`.

### 4. parcadei/Continuous-Claude-v3 — pre-compact daemon
Hooks into `pre_compact` to write a ~400-token YAML handoff that the
`session_start_continuity.py` hook replays on the next session.

- Daemon socket path is computed from `md5(project_dir)` → same socket per
  project, cross-platform (Unix sockets on \*nix, TCP-with-port-from-hash on
  Windows). `.claude/hooks/session_start_continuity.py:27-49`.
- PR #153 fixes a real-world footgun: `tryStartDaemon` had to switch to
  `spawn+detach` to survive parent timeouts.
- PR #157 ports `ensureMemoryDaemon` to a TypeScript session-start hook —
  shows the daemon pattern stabilizing across the ecosystem.

### 5. rohitg00/pro-workflow — the cron-tick driver (★ pick of the survey)
The cleanest reference implementation of "run Claude on a cron" found:

```js
// scripts/research-tick.js  (~lines 44-73, paraphrased)
if (fs.existsSync(path.join(home, 'STOP'))) process.exit(0);   // kill-switch
const pending = db.prepare(
  "SELECT id, prompt FROM seeds WHERE status='pending' LIMIT 1"
).get();
if (!pending) process.exit(0);
const child = spawn('claude', ['-p', pending.prompt], {
  timeout: 10 * 60 * 1000,
  stdio: ['ignore', logStream, logStream],
});
```

A bare cron line is enough to drive it: `*/10 * * * * node ~/pro-workflow/scripts/research-tick.js`.
The companion `config-watcher.js` listens for `ConfigChange` (Claude Code
2.1.49+) on `settings.json`, `hooks.json`, `.claudeignore` and rotates a
100KB log. PR #53 ("wiki knowledge base + auto-research loop + multi-LLM
council") is where the loop was hardened.

### 6. rohitg00/awesome-claude-code-toolkit — Jarvis / DiscoClaw bridge
The toolkit references but doesn't vendor **Jarvis** (now its own repo —
[Ramsbaby/jarvis](https://github.com/Ramsbaby/jarvis)): 76 scheduled tasks,
12 AI teams, Discord bot front, headless `claude -p` execution, 4-layer
self-healing, macOS LaunchAgents (not cron, but equivalent). Eight
orchestration agents are catalogued: Task Coordinator, Context Manager,
Workflow Director, Agent Installer, Knowledge Synthesizer, Performance
Monitor, Error Coordinator, Multi-Agent Coordinator.

### 7. Yeachan-Heo/oh-my-claudecode — guidance schema + runtime overlays
Markers `<!-- OMX:RUNTIME:START --> … <!-- OMX:RUNTIME:END -->` and
`<!-- OMX:TEAM:WORKER:START --> … <!-- OMX:TEAM:WORKER:END -->` partition
agent prompts so the *contract* is hand-editable but the *runtime overlay* is
machine-injected. Up to 6 concurrent children via `spawn_agent`, each with an
isolated context. `AGENTS.md:6-20, 62-90`. PR #3020 ("handle Claude Code
v2.1.x banner and Enter swallow stalls") shows the painful reality of driving
the CLI as a child process.

### 8. nyldn/claude-octopus — parallel fan-out + consensus gate
8-way fan-out to Codex/Gemini/Copilot/Qwen/Ollama/Perplexity/OpenRouter/OpenCode
through `/octo:parallel`, with a **75% consensus quality gate**. PR #379
(`wait on done markers with configurable deadline`) and #378 (`embrace,
discover: dispatch workflows directly`) show the synchronization layer
hardening. The repo also has a real `schedule: cron: '0 2 * * *'` nightly
build workflow.

### 9. anthropics/claude-code-action — mode detector
Single TS entrypoint `src/entrypoints/run.ts`. `detectMode()` auto-picks
**tag mode** (builds prompt from issue/PR/comment/diff/CI context) vs
**agent mode** (raw user prompt). Base-action is published standalone
(`@anthropic-ai/claude-code-base-action`) with a CLI install retry loop
(3 attempts, 5s backoff). Recent PRs are mostly hardening (Bun pin #1312,
symlink dereffing in snapshots #1186) — no scheduled-trigger work, the
schedule lives in the consumer's workflow YAML.

### 10. 21st-dev/1code — desktop multi-agent
Electron + tRPC + Drizzle SQLite. `sub_chats` table isolates tabs; each tab
streams via tRPC subscription; Claude Code SDK is dynamically imported and
resumed via `sessionId`. PR #203 (CLI-parity built-in subagents shipped to
embedded SDK) is the orchestration headline.

### 11. golutra/golutra — multi-CLI dispatch
Tauri (Rust + Vue). `terminalOrchestratorStore.ts:30-100` —
`ensureMemberSession()` + `openMemberTerminal()` per AI "member" with a
serialized `enqueueTerminalDispatch(request)` queue and toast-driven
resource-limit detection. PR #137 (`Reconcile global skill registry from
disk on startup`) shows the persistence layer maturing.

### Quick hits

- **stellarlinkco/myclaude** — `omo` (Orchestrator Multi-Option) routes to
  Codex / Claude / Gemini / OpenCode by complexity; `do` is a 5-phase feature
  pipeline.
- **vijaythecoder/awesome-claude-agents** — `@agent-team-configurator`
  auto-detects stack (package.json / composer.json / go.mod / Gemfile) and
  rewrites CLAUDE.md with a timestamped "AI Team Configuration" block.
- **shinpr/claude-code-workflows** — `/recipe-implement`,
  `/recipe-front-design`, `/recipe-fullstack-build` with `design-sync`
  cross-layer validation.
- **hesreallyhim/awesome-claude-code** — has its own scheduled GH Action
  (`update-repo-ticker.yml` cron `0 */3 * * *`, plus `check-repo-health.yml`).

---

## Part 2 — Projects that *facilitate* orchestration and cron

These aren't agent collections; they're the plumbing.

### Cron / scheduling

| Project | Mechanism | Notes |
|---|---|---|
| **[phildougherty/claudecron](https://github.com/phildougherty/claudecron)** | MCP server | Claude registers cron jobs by calling MCP tools — agent self-schedules. |
| **[tonybentley/claude-mcp-scheduler](https://github.com/tonybentley/claude-mcp-scheduler)** | Claude API + local MCP | Remote prompts on cron; tool calls run locally for context. |
| **[kylemclaren/claude-tasks](https://github.com/kylemclaren/claude-tasks)** | TUI (Bubble Tea) | 6-field cron (second granularity), Discord + Slack webhooks, usage thresholds. |
| **[jshchnz/claude-code-scheduler](https://github.com/jshchnz/claude-code-scheduler)** | OS-native | launchd / crontab / Task Scheduler — same UX everywhere. |
| **[builderz-labs/mission-control](https://github.com/builderz-labs/mission-control)** | Natural language → cron | "every morning at 9am" parsed to cron; template-clone pattern keeps the original and spawns dated children. |
| **anthropics/claude-code#30646, #30649** | Feature requests | Official scheduled-skill support is *not* shipped yet — the gap these tools fill. |

### Multi-agent orchestration runtimes

| Project | Distinguishing feature |
|---|---|
| **[swarmclawai/swarmclaw](https://github.com/swarmclawai/swarmclaw)** | Self-hosted runtime; durable memory + schedules + 23+ LLM providers. |
| **[ComposioHQ/agent-orchestrator](https://github.com/ComposioHQ/agent-orchestrator)** | Plans tasks → spawns agents → handles CI fixes, merge conflicts, code reviews autonomously. |
| **[awslabs/cli-agent-orchestrator](https://github.com/awslabs/cli-agent-orchestrator)** | Adapters for Claude Code / Kiro / Codex / Gemini / Kimi / Copilot CLI / OpenCode / Amazon Q. Cron-style Flows + headless agents in CI. |
| **[jayminwest/overstory](https://github.com/jayminwest/overstory)** | Persistent coordinator (`ov coordinator start`); spawns headless Claude workers communicating via stream-json over stdout. |
| **[nwyin/hive](https://github.com/nwyin/hive)** | `--headless` Queen creates issues directly from `-p` prompt then exits — minimal autopilot loop. |
| **[andyrewlee/awesome-agent-orchestrators](https://github.com/andyrewlee/awesome-agent-orchestrators)** | Up-to-date index of the whole category. |

### Git-worktree fan-out (parallel agent isolation)

- **[Vibe Kanban](https://vibekanban.com/)** ([source](https://github.com/BloopAI/vibe-kanban)) — Kanban UI over worktrees. Bloop announced a 2026 shutdown; the project remains OSS / community-maintained.
- **Claude Squad, Conductor, Crystal, Gastown, Nimbalyst** — all solve the same "N agents, N worktrees, one main repo" problem with different UIs ([roundup](https://nimbalyst.com/blog/best-git-worktree-tools-ai-coding-2026/)).

### Discord / chat bridges (often the front for a cron-driven backend)

- **[Ramsbaby/jarvis](https://github.com/Ramsbaby/jarvis)** — 99 automation scripts, RAG insight layer, runs 24/7 on a Claude Max subscription, $0 extra API spend.
- **[unohee/OpenSwarm](https://github.com/unohee/OpenSwarm)** — Discord control surface for a Worker/Reviewer pair pipeline; pulls Linear issues; LanceDB long-term memory.
- **[disclaude/app](https://github.com/disclaude/app)**, **[zebbern/claude-code-discord](https://github.com/zebbern/claude-code-discord)**, **[ebibibi/claude-code-discord-bridge](https://github.com/ebibibi/claude-code-discord-bridge)**, **[chadingTV/claudecode-discord](https://github.com/chadingTV/claudecode-discord)** — varying takes on "Claude Code session over Discord", several with `SchedulerCog`-style 30s master loops and `POST /api/tasks` self-registration.

### Official / first-party

- **[anthropics/claude-code-action](https://github.com/anthropics/claude-code-action)** — drop-in GitHub Action; pair with `on: schedule: cron:` for any cadence.
- **Claude Code Routines** ([docs](https://code.claude.com/docs/en/routines)) — Anthropic-managed cloud scheduling, min interval 1h, custom cron via `/schedule update`. The first-party answer to the cron gap.

---

## Patterns worth importing into this repo

If we ever want to add Claude-driven orchestration to `prediction_markets`:

1. **Cron-tick driver** (steal from `rohitg00/pro-workflow`): a Go binary that
   reads pending tasks from Postgres, spawns `claude -p`, respects a STOP file.
   Drop-in for nightly market hygiene jobs.
2. **YAML handoff** (steal from `parcadei/Continuous-Claude-v3`): pre-compact
   hook writes a small YAML summarizing what was learned; next session loads it.
   Useful for long-running data-science investigations.
3. **GitHub Action schedule** (already supported by
   `anthropics/claude-code-action`): nightly orderbook anomaly report into an
   Issue costs almost nothing to wire up.
4. **Soft-enforcement nudges** (from `barkain/...orchestration`): per-turn
   counters that resist mis-use without breaking the loop — good fit for a
   tightly-scoped agent that should always delegate before editing.

---

## Sources

- [ruvnet/ruflo](https://github.com/ruvnet/ruflo)
- [wshobson/agents](https://github.com/wshobson/agents)
- [barkain/claude-code-workflow-orchestration](https://github.com/barkain/claude-code-workflow-orchestration)
- [Yeachan-Heo/oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode)
- [21st-dev/1code](https://github.com/21st-dev/1code)
- [parcadei/Continuous-Claude-v3](https://github.com/parcadei/Continuous-Claude-v3)
- [golutra/golutra](https://github.com/golutra/golutra)
- [rohitg00/pro-workflow](https://github.com/rohitg00/pro-workflow)
- [rohitg00/awesome-claude-code-toolkit](https://github.com/rohitg00/awesome-claude-code-toolkit)
- [anthropics/claude-code-action](https://github.com/anthropics/claude-code-action)
- [stellarlinkco/myclaude](https://github.com/stellarlinkco/myclaude)
- [nyldn/claude-octopus](https://github.com/nyldn/claude-octopus)
- [hesreallyhim/awesome-claude-code](https://github.com/hesreallyhim/awesome-claude-code)
- [vijaythecoder/awesome-claude-agents](https://github.com/vijaythecoder/awesome-claude-agents)
- [shinpr/claude-code-workflows](https://github.com/shinpr/claude-code-workflows)
- [phildougherty/claudecron](https://github.com/phildougherty/claudecron)
- [tonybentley/claude-mcp-scheduler](https://github.com/tonybentley/claude-mcp-scheduler)
- [kylemclaren/claude-tasks](https://github.com/kylemclaren/claude-tasks)
- [jshchnz/claude-code-scheduler](https://github.com/jshchnz/claude-code-scheduler)
- [builderz-labs/mission-control](https://github.com/builderz-labs/mission-control)
- [swarmclawai/swarmclaw](https://github.com/swarmclawai/swarmclaw)
- [ComposioHQ/agent-orchestrator](https://github.com/ComposioHQ/agent-orchestrator)
- [awslabs/cli-agent-orchestrator](https://github.com/awslabs/cli-agent-orchestrator)
- [jayminwest/overstory](https://github.com/jayminwest/overstory)
- [nwyin/hive](https://github.com/nwyin/hive)
- [andyrewlee/awesome-agent-orchestrators](https://github.com/andyrewlee/awesome-agent-orchestrators)
- [Ramsbaby/jarvis](https://github.com/Ramsbaby/jarvis)
- [unohee/OpenSwarm](https://github.com/unohee/OpenSwarm)
- [Vibe Kanban](https://vibekanban.com/)
- [Claude Code Routines (docs)](https://code.claude.com/docs/en/routines)
- [Feature request #30646 — Scheduled / cron task support](https://github.com/anthropics/claude-code/issues/30646)
