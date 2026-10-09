# Fleetglass: competitive landscape

Status: research snapshot, 2026-10-08. Public web pages only (vendor sites, docs, pricing pages, changelogs, GitHub,
review aggregators, HN). No accounts, no logins. Prices are list prices as published or as reported by third-party
pricing guides on that date; they move often. Anything not confirmed by a source is marked **(unverified)**.
Third-party pricing aggregators (costbench, cubeapm, hyperping, etc.) are competitors or affiliates of the products
they describe: treat their numbers as indicative.

Scope reference: [PRODUCT.md](PRODUCT.md) (11 coverage domains, self-hosted first, open core, cockpit design).

---

## 0. Executive summary

- **The niche is real but no longer empty.** Since mid-2026 Claude Code and Codex CLI both emit OpenTelemetry, and the
  big platforms ship prebuilt "coding agent" views: Datadog Agent Console (preview), Grafana Cloud Claude Code
  integration, Dynatrace and AWS CloudWatch Codex dashboards, Dash0 AI Coding Insights ($10/dev/month). All of them are
  SaaS, organisation/developer-centric (who spent what), and none ties agent sessions to the *machine* they run on.
- **On the server side, the reference products are free.** Beszel (MIT, ~26k stars, ~10-15 MB agent) and Uptime Kuma
  (MIT, ~92k stars) define what self-hosters expect at $0; Netdata gives 5 nodes free and unlimited homelab nodes for
  $90/year, with AI troubleshooting and an MCP server.
- **On the agent side, the reference products are tiny local tools.** ccusage (MIT, ~19k stars, 16+ agent CLIs),
  Claude-Code-Usage-Monitor (terminal burn-rate forecasts), CodexBar (macOS menu bar quotas), Anthropic's own Agent View
  (`claude agents`, local, per-user, no tokens or cost). None is a server dashboard, none alerts, none keeps history
  across machines.
- **Fleetglass's defensible position**: the only self-hosted product where an agent session is a first-class object
  linked to processes, CPU/RAM, disk, worktrees, services, incidents and quota pace on the same timeline. Lead with
  that, match Beszel/Uptime Kuma on basics, and do not try to out-feature Datadog/Grafana.
- **Pricing**: ~€9/server/month is above Netdata Business ($4.50/node) and Beszel (free) and only makes sense if the
  agent layer is the product. Recommendation in section 5.

---

## 1. Product-by-product notes

Legend: **Pos** positioning / target user, **Dep** deployment, **Price** model and entry price, **Std** standout
features, **UX** standout UX or visual patterns, **Weak** notable weaknesses or complaints.

### 1.1 Infrastructure monitoring

#### Datadog
- **Pos**: full-stack SaaS observability for mid-size to enterprise engineering orgs.
- **Dep**: SaaS only, agent on hosts.
- **Price**: per product per host. Infrastructure Pro $15/host/month annual ($18 on demand), Enterprise $23 ($27);
  APM $31/host extra; custom metrics and logs billed separately.
  [last9](https://last9.io/blog/datadog-pricing-all-your-questions-answered/),
  [signoz](https://signoz.io/blog/datadog-pricing/)
- **Std**: breadth; Watchdog automatic anomaly detection and root-cause analysis with no configuration
  ([Datadog blog](https://www.datadoghq.com/blog/datadog-watchdog-automated-root-cause-analysis/)); **Agent Console**
  (preview) for coding agents (Claude Code, Cursor, Copilot): usage, spend, active users, sessions, correlated with
  engineering metrics like time to merge, fed by the Anthropic Usage & Costs integration or OTLP
  ([docs](https://docs.datadoghq.com/ai_agents_console/), [setup](https://docs.datadoghq.com/ai_agents_console/setup/)).
- **UX**: unified tagging across products; event overlays on graphs **(unverified, general knowledge)**.
- **Weak**: bill shock is a recurring HN genre: $65M customer bill
  ([HN](https://news.ycombinator.com/item?id=35837330)), "our DD bill was more than the service"
  ([HN](https://news.ycombinator.com/item?id=35837965)), $83k/year renewal
  ([HN](https://news.ycombinator.com/item?id=41357726)); custom metrics pricing is a known trap
  ([signoz](https://signoz.io/blog/community-update-24/)). Agent Console still in preview with limited session
  reconstruction ([Dash0 comparison](https://www.dash0.com/comparisons/claude-code-monitoring-tools)).

#### Grafana Cloud (and OSS Grafana/Prometheus)
- **Pos**: composable observability (metrics, logs, traces, profiles) for engineers who like to build dashboards.
- **Dep**: OSS self-hosted or SaaS.
- **Price**: Free tier (10k active series, 50 GB logs/traces/profiles, 14-day retention, 3 users), Pro from $19/month
  + usage, Enterprise from $25k/year. [CloudZero](https://www.cloudzero.com/blog/grafana-cloud-pricing/),
  [motadata](https://www.motadata.com/blog/grafana-cloud-pricing)
- **Std**: **Claude Code integration** with prebuilt dashboards (cost, tokens, tools, sessions, active time, lines,
  commits, per-user breakdown) ([Grafana docs](https://grafana.com/docs/grafana-cloud/observe-and-act/monitor-infrastructure/integrations/integration-reference/integration-claude-code/));
  community dashboards for Claude Code ([25052](https://grafana.com/grafana/dashboards/25052-claude-code/)) and Codex
  ([25338](https://grafana.com/grafana/dashboards/25338-codex/)).
- **UX**: infinitely flexible panels; the de facto lingua franca of dashboards.
- **Weak**: steep learning curve, PromQL, dashboard sprawl, hard to share with people who do not write queries
  ([G2](https://www.g2.com/products/grafana-labs/reviews?qs=pros-and-cons)). Grafana stack costs 500 MB-1 GB+ RAM
  self-hosted ([techfuelhq](https://techfuelhq.com/homelab/beszel-vs-netdata-vs-glances-2026/)). Claude Code view lacks a
  purpose-built session experience ([Dash0](https://www.dash0.com/comparisons/claude-code-monitoring-tools)).

#### New Relic
- **Pos**: full-stack observability, consumption pricing.
- **Dep**: SaaS.
- **Price**: perpetual free tier (100 GB/month ingest, 1 full user); then $0.40/GB and $99-349 per full user/month.
  [New Relic](https://newrelic.com/info/pricing), [signoz](https://signoz.io/guides/new-relic-pricing/)
- **Std**: generous free ingest; all capabilities available on free.
- **Weak**: per-user pricing for full users drives cost; overkill for one server
  ([middleware](https://middleware.io/blog/new-relic-pricing/)).

#### Netdata
- **Pos**: per-second, zero-config metrics with ML anomaly detection; from homelab to enterprise.
- **Dep**: open agent on each node + Netdata Cloud SaaS (or Enterprise on-prem, minimum 200 nodes).
- **Price**: Community free up to 5 nodes and 1 custom dashboard; Homelab $90/year unlimited nodes (non-commercial
  self-certification); Business $4.50/node/month annual. [pricing](https://www.netdata.cloud/pricing/),
  [homelab](https://www.netdata.cloud/homelab/)
- **Std**: thousands of metrics at 1 s; 18 ML models per metric; AI Co-Engineer; **MCP server** so Claude/ChatGPT can
  query metrics, anomalies, alerts and logs ([MCP](https://www.netdata.cloud/features/aiml/mcp/),
  [Cloud MCP](https://www.netdata.cloud/blog/netdata-cloud-mcp-server/)).
- **UX**: auto-generated charts for everything detected, no configuration.
- **Weak**: footprint 100-350 MB RAM per agent vs 10-15 MB for Beszel
  ([techfuelhq](https://techfuelhq.com/homelab/beszel-vs-netdata-vs-glances-2026/)); memory-leak reports
  ([#21449](https://github.com/netdata/netdata/issues/21449)); the v2 dashboard is closed source (NCUL1), which got the
  package removed from Debian and flagged unfree on NixOS ([Debian](https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=1045145),
  [isitreallyfoss](https://isitreallyfoss.com/projects/netdata/), [#16035](https://github.com/netdata/netdata/issues/16035)).
  Density of charts can drown the signal **(unverified, reviewer opinion)**.

#### Beszel
- **Pos**: lightweight multi-server monitoring for homelabs and small fleets.
- **Dep**: self-hosted hub (PocketBase) + agent; MIT; ~26k GitHub stars.
  [GitHub](https://github.com/henrygd/beszel)
- **Price**: free.
- **Std**: CPU, memory, disk, I/O, network, temperatures, fans, GPU (Nvidia/AMD/Intel), S.M.A.R.T., ZFS, Docker per
  container, alerts, multi-user, OAuth/OIDC, automatic backups, API; systemd service view with last 200 journal lines
  ([systemd docs](https://beszel.dev/guide/systemd)).
- **UX**: "is my server healthy" in five minutes, no query language
  ([techfuelhq](https://techfuelhq.com/homelab/beszel-vs-netdata-vs-glances-2026/)).
- **Weak**: no uptime probes, status page, incidents, security, backups or AI agent layer **(scope inferred from
  README)**; systemd view needs systemd 243+ and skips oneshot units
  ([#2108](https://github.com/henrygd/beszel/issues/2108), [systemd docs](https://beszel.dev/guide/systemd));
  rootless Docker limits ([docs #85](https://github.com/henrygd/beszel-docs/issues/85)).

#### Zabbix
- **Pos**: enterprise-grade open-source monitoring (network, servers, SNMP).
- **Dep**: self-hosted; GPL, no paid edition; paid support/consulting.
  [wz-it](https://wz-it.com/en/knowledge/monitoring/zabbix-vs-checkmk/)
- **Std**: templates, low-level discovery (nested in 7.4), host wizard, network maps, live dashboard editing
  ([What's new 7.4](https://www.zabbix.com/documentation/7.4/en/manual/introduction/whatsnew)).
- **Weak**: dated, unintuitive UI and steep learning curve are the top complaint across G2/Capterra/TrustRadius
  ([G2](https://www.g2.com/products/zabbix/reviews?qs=pros-and-cons)).

#### Checkmk
- **Pos**: hybrid IT monitoring, strong auto-discovery, European vendor.
- **Dep**: self-hosted Community (ex-Raw, GPLv2) or paid; Cloud SaaS.
- **Price**: Community free; Pro from €190/month, Ultimate from €275/month, Cloud from €240/month (annual).
  [wz-it](https://wz-it.com/en/knowledge/monitoring/zabbix-vs-checkmk/)
- **Std**: automatic host/service discovery; Agent Bakery (packaged agents with plugins, automatic updates in
  commercial editions) ([docs](https://docs.checkmk.com/latest/en/agent_deployment.html)); local checks
  ([docs](https://docs.checkmk.com/latest/en/localchecks.html)).
- **Weak**: enterprise feel and price jump between free and paid **(unverified, inferred from price list)**.

#### Dynatrace
- **Pos**: enterprise APM/AIOps.
- **Dep**: SaaS or managed.
- **Price**: platform subscription drawn against a rate card; Full-Stack $0.01 per memory-GiB-hour (~$58/month for an
  8 GiB host); Infrastructure ~ $29/host/month. [last9](https://last9.io/blog/dynatrace-pricing/),
  [monitoringcost](https://monitoringcost.com/dynatrace-pricing)
- **Std**: Davis AI causal analysis **(unverified)**; Codex CLI monitoring app on the hub
  ([Dynatrace Hub](https://www.dynatrace.com/hub/detail/openai-codex/)).
- **Weak**: cost scales with host RAM, enterprise-only fit.

### 1.2 Uptime, status pages, incidents, cron

#### Better Stack
- **Pos**: modern all-in-one uptime + on-call + incidents + status pages + logs for startups.
- **Dep**: SaaS.
- **Price**: free (10 monitors, 3-minute checks, 1 status page); Responder licence $29/month annual ($34 monthly);
  add-ons (50 monitors $25, Slack workflows, extra status page features). [hyperping](https://hyperping.com/blog/betterstack-pricing),
  [modern-datatools](https://www.modern-datatools.com/tools/better-stack/pricing)
- **Std**: incident timeline starts automatically when a monitor fails; screenshots and second-by-second timeline;
  post-mortems generated from the timeline ([hyperping comparison](https://hyperping.com/blog/betterstack-vs-site24x7-vs-hyperping)).
  Also ingests Claude Code OTel ([docs](https://betterstack.com/docs/logs/claude-code/)).
- **UX**: widely cited as the best-looking uptime/status product **(unverified, general reputation)**.
- **Weak**: modular pricing adds up; status page features as per-page add-ons; slow log pages reported
  ([G2](https://www.g2.com/products/better-stack/reviews?qs=pros-and-cons)).

#### Uptime Kuma
- **Pos**: the default self-hosted uptime monitor.
- **Dep**: self-hosted, MIT, ~92k stars, v2. [GitHub](https://github.com/louislam/uptime-kuma)
- **Price**: free.
- **Std**: HTTP(S), keyword, JSON query, TCP, ping, DNS, push, Docker container monitors; 20 s intervals; multiple
  status pages with custom domains; certificate info; 90+ notification services (91 in v2)
  ([GitHub](https://github.com/louislam/uptime-kuma), [v2 notes](https://uptimekuma.io/uptime-kuma-v2-whats-new/)).
- **UX**: "fancy, reactive, fast" UI; heartbeat bar per monitor.
- **Weak**: SQLite contention beyond a few hundred monitors, ~15 s dashboard load at ~900 monitors
  ([#6181](https://github.com/louislam/uptime-kuma/issues/6181), [hyperping](https://hyperping.com/blog/best-uptime-kuma-alternatives));
  weak multi-user, no SSO, API requested for years ([hyperping](https://hyperping.com/blog/best-uptime-kuma-alternatives)).
  Lesson for Fleetglass, which also uses SQLite.

#### Checkly
- **Pos**: synthetic monitoring "as code" (Playwright) for product teams.
- **Dep**: SaaS, private locations on Team.
- **Price**: Hobby free (10 uptime monitors, 1k browser runs, 10k API runs, 6 locations); Starter $24/month annual;
  Team $64/month. [cubeapm](https://cubeapm.com/blog/checkly-pricing-review/)
- **Std**: checks defined in TypeScript, versioned in Git, deployed via CI
  ([modern-datatools](https://www.modern-datatools.com/tools/checkly)).
- **Weak**: browser-run metering; not infra monitoring.

#### Atlassian Statuspage
- **Pos**: the incumbent hosted status page.
- **Price**: free (100 subscribers, 25 components, 2 metrics); Hobby $29; Startup $99; Business $399; Enterprise
  $1,499/month. [costbench](https://costbench.com/software/network-monitoring/atlassian-statuspage/free-plan/)
- **Std**: subscriber notifications, component model, third-party component embedding **(unverified)**.
- **Weak**: steep jumps between tiers ([hyperping](https://hyperping.com/blog/statuspage-pricing)); no monitoring of its own.

#### incident.io
- **Pos**: Slack-native incident response + on-call + status pages.
- **Price**: Basic free (up to 5 users); Team $19/user/month ($15 annual) + on-call add-on; Pro $25/user + $20 on-call.
  [spike.sh](https://blog.spike.sh/incident-io-pricing-breakdown-2026/)
- **Std**: `/inc` commands in Slack, automatic timeline capture, service catalog in the incident channel,
  status page update on resolve ([incident.io](https://incident.io/blog/slack-native-incident-management-evaluation)).
- **Weak**: Slack/Teams only; status page limited in base tiers ([vigileshq](https://vigileshq.com/blog/incident-io-review)).

#### PagerDuty
- **Pos**: enterprise on-call and alert routing.
- **Price**: free up to 5 users (100 international SMS/calls); Professional $21/user/month annual; Business $41.
  [costbench](https://costbench.com/software/developer-tools/pagerduty/)
- **Weak**: expensive, AIOps noise reduction is a paid add-on, alert fatigue, dense UI
  ([spike.sh](https://spike.sh/blog/pagerduty-review-for-alerting-2026/),
  [incident.io interviews](https://incident.io/blog/pagerduty-alternatives-top-pain-points)).

#### Healthchecks.io
- **Pos**: dead-man's-switch for cron jobs and background tasks.
- **Dep**: SaaS or self-hosted (BSD). [GitHub](https://github.com/healthchecks/healthchecks)
- **Price**: Hobbyist free (20 checks); Business $20/month (100); Business Plus $80/month (1,000).
  [foursight](https://foursight.cloud/compare/healthchecks-pricing)
- **Std**: cron expressions with time zone, grace time, `start`/`success`/`fail` signals measuring run duration,
  25+ integrations, read-only team access ([docs](https://healthchecks.io/docs/configuring_checks/)).
- **UX**: one-line `curl` integration; minimal, honest UI **(unverified, opinion)**.

#### Cronitor
- **Pos**: cron, heartbeat and uptime monitoring for developers.
- **Price**: free 5 monitors; then $2/monitor + $5/user per month. [Cronitor](https://cronitor.io/pricing),
  [cronalert](https://cronalert.com/compare/cronitor)
- **Std**: auto-discovery of crontab entries via CLI **(unverified)**.
- **Weak**: per-monitor pricing scales poorly ([hyperping](https://hyperping.com/blog/best-cronitor-alternatives)).

### 1.3 Product-grade dashboards (UI quality references)

| Product | What to learn | Sources |
|---|---|---|
| Vercel Observability | Query builder with AI query prompting; anomaly alerts on error and usage (5-minute window vs 24 h baseline, >4 standard deviations); spend management with alerts at 50/75/100 % and optional automatic pause; usage-based ($1.20 / 1M events, base fee removed) | [docs](https://vercel.com/docs/observability/observability-plus), [anomaly alerts GA](https://vercel.com/changelog/anomaly-alerts-ga), [alerts](https://vercel.com/docs/alerts), [spend](https://vercel.com/blog/introducing-spend-management-realtime-usage-alerts-sms-notifications), [base fee](https://vercel.com/changelog/no-base-fee-for-observability-plus) |
| Railway | Customisable observability dashboard (drag/resize widgets), CPU/mem/net/disk across services side by side, HTTP p50-p99 and 5xx panels, live log panels with saved filters, project canvas updated live without reload | [docs](https://docs.railway.com/observability), [changelog](https://railway.com/changelog/2026-07-31-railway-plugin-for-chatgpt) |
| Fly.io | Managed Grafana + Prometheus-compatible (VictoriaMetrics) store, free, ~15-day retention, prebuilt dashboards for every app | [docs](https://fly.io/docs/monitoring/metrics/) |
| Cloudflare dashboard | Navigation reorganised by job to be done rather than product name; "suspicious activity" surfaced at the top of analytics | [changelog](https://developers.cloudflare.com/changelog/post/2026-01-15-networking-navigation-update/), [InfoQ](https://www.infoq.com/news/2026/05/cloudflare-security-dashboard/) |
| Sentry | Grouping by fingerprint to avoid alert floods; suspect commits via git blame; release health and regression detection; Seer AI add-on $40/contributor | [grouping](https://mintlify.wiki/getsentry/sentry/features/issue-grouping), [suspect commits](https://docs.sentry.io/product/issues/suspect-commits/), [pricing](https://last9.io/blog/sentry-pricing/) |
| PostHog | Transparent usage pricing with large free tiers (1M events, 100k LLM events), unlimited seats, per-product meters; LLM analytics with per-user cost | [pricing](https://posthog.com/pricing), [rybbit](https://rybbit.com/blog/posthog-pricing) |
| Linear | Speed as a design decision; Cmd+K command menu; single-letter, discoverable shortcuts; restrained, uncluttered UI | [925studios](https://www.925studios.co/blog/linear-design-breakdown-saas-ui-2026), [performance.dev](https://performance.dev/how-is-linear-so-fast-a-technical-breakdown) |

### 1.4 AI / LLM observability and agent ops (the differentiator)

#### Application LLM observability (adjacent, not direct competitors)

| Product | Pos / Dep | Price (entry) | Standout | Weakness | Sources |
|---|---|---|---|---|---|
| Langfuse | OSS LLM tracing, evals, prompts; MIT, self-host free and unlimited | Cloud Hobby free (50k units), Core $29, Pro $199 | Claude Code integration via Stop hook: prompts, turns, tokens, cost, tools, subagents grouped by session | App-centric; no machine or quota view | [compare](https://langfuse.com/compare/langsmith), [pricing guide](https://www.cekura.ai/blogs/langfuse-pricing), [Claude Code](https://langfuse.com/integrations/developer-tools/claude-code), [coding agents](https://langfuse.com/resources/engineering/coding-agent-tracing) |
| Helicone | Proxy/gateway + observability; Apache-2.0 | Hobby free (10k req), Pro $79 | One-line proxy integration | Acquired by Mintlify (Mar 2026), maintenance mode | [chatforest](https://chatforest.com/reviews/helicone-llm-observability-gateway/), [futureagi](https://futureagi.com/blog/helicone-alternatives-2026/) |
| LangSmith | LangChain's tracing/evals; SaaS (self-host enterprise) | Developer free (1 seat, 5k traces), Plus $39/seat | Deep LangGraph integration | Per-seat + per-trace billing | [zenml](https://www.zenml.io/blog/langsmith-pricing), [inference.net](https://inference.net/content/langsmith-pricing/) |
| Arize Phoenix / AX | OTel-native tracing + evals; Phoenix ELv2 (source-available) | Phoenix free self-host; AX Pro $50/month | OpenInference conventions | ELv2 not OSI open source | [GitHub](https://github.com/arize-ai/phoenix), [usagepricing](https://www.usagepricing.com/blueprint/arize-ai) |
| Braintrust | Eval-first LLM platform; SaaS | Starter free, Pro $249/month | Experiments, scoring | Expensive step to Pro | [truefoundry](https://www.truefoundry.com/blog/braintrust-pricing) |
| OpenLLMetry / Traceloop | OTel instrumentation for LLMs, Apache-2.0; Traceloop SaaS | free < 50k spans | Vendor-neutral, 25+ backends | Acquired by ServiceNow (2026) | [Traceloop pricing](https://www.traceloop.com/pricing), [aicalculatorpro](https://aicalculatorpro.com/ai-stack/llm-observability/traceloop/) |
| AgentOps | Agent framework tracing; MIT SDK and app | Basic free (5k events), Pro from $40 | Session replay, time-travel debugging, cost across 400+ LLMs | For agents you build, not CLI agents you run | [inference.net](https://inference.net/content/agentops-alternatives/), [therundown](https://www.therundown.ai/tools/agentops) |
| PostHog LLM analytics | Product analytics with LLM surface | 100k LLM events free | Per-user cost | App-centric | [pricing](https://posthog.com/pricing) |

Takeaway: these products observe LLM calls made *by applications you write*. They answer "what did my app's model call
cost". They do not watch a Claude Code or Codex process on a server, its quota windows, or the machine.

#### Provider consoles (first-party)

| Product | What it shows | Gaps | Sources |
|---|---|---|---|
| Anthropic Console + Admin API | Usage (tokens: uncached input, cache read/write, output) by key/workspace/model; cost report in USD by day; Claude Code Analytics API (sessions, commits, PRs, lines) free with Admin API | Admin key required (org admins); seat plans get no per-token data; excludes Bedrock/Vertex; no live state | [Usage & Cost API](https://platform.claude.com/docs/en/manage-claude/usage-cost-api), [Claude Code Analytics API](https://platform.claude.com/docs/en/manage-claude/claude-code-analytics-api), [Torii](https://www.toriihq.com/articles/five-claude-code-usage-dashboards-and-monitoring-tools) |
| OpenAI usage dashboard + Usage/Costs API | Tokens per minute/hour/day by model, key, project, user; Costs endpoint reconciles to invoice | API usage only (not ChatGPT-plan Codex quotas); alerts thin | [help](https://help.openai.com/en/articles/10478918-api-usage-dashboard), [cookbook](https://cookbook.openai.com/examples/completions_usage_api) |
| Claude Code native telemetry | OTel metrics: session.count, token.usage, cost.usage, active_time, lines, commits, PRs, edit decisions; 20+ events (api_request, api_error, tool_result, permission_mode_changed, ...); content redacted by default; repository attributes opt-in | **No quota / rate-limit metric**; exporter must be configured per user | [Claude Code monitoring docs](https://code.claude.com/docs/en/monitoring-usage) |
| Claude Code Agent View (`claude agents`) | Research preview (May 2026): table of background sessions across projects on the machine, state (working, needs input, idle, completed, failed), Haiku one-line summary, age, PR status, "waiting X m" | Local terminal only, per user, background sessions only, **no tokens or cost**, no history, no alerts | [docs](https://code.claude.com/docs/en/agent-view) |
| Codex CLI telemetry | OTel metrics/logs (tokens per turn incl. reasoning and cached, latency, tool calls, approvals); session JSONL `token_count` events carry `rate_limits.primary/secondary.used_percent`, `window_minutes`, `resets_at`, `plan_type`; `/status` shows 5 h and weekly windows | No first-party dashboard for self-hosted servers | [CloudWatch docs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/coding-agents-codex.html), [tokenatlas #89](https://github.com/Magnus-Gille/tokenatlas/issues/89), [codex-ratelimit](https://github.com/xiangz19/codex-ratelimit) |

#### Coding-agent monitors (closest direct competitors)

| Tool | Approach | Price | Strength | Gap | Sources |
|---|---|---|---|---|---|
| ccusage | CLI parsing local agent logs; MIT, ~19k stars | free | 16+ CLIs (Claude Code, Codex, Gemini, Copilot CLI, ...); daily/weekly/monthly, per session, 5-hour blocks, per project, per model; statusline; offline pricing | One machine, one user, no UI, no history store, no alerts | [GitHub](https://github.com/ryoppippi/ccusage), [Torii](https://www.toriihq.com/articles/five-claude-code-usage-dashboards-and-monitoring-tools) |
| Claude-Code-Usage-Monitor | Python TUI | free | Live burn rate, P90-based prediction of 5-hour block exhaustion, plan auto-detection | Terminal only, no history export, no team view | [GitHub](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor), [Torii](https://www.toriihq.com/articles/five-claude-code-usage-dashboards-and-monitoring-tools) |
| CodexBar | macOS menu bar app, MIT | free | 5-hour and weekly windows for Codex, Claude and many providers, warnings before running low, reuses CLI logins | Desktop macOS only, not a server | [GitHub](https://github.com/steipete/CodexBar), [site](https://codexbar.app/) |
| claude-agents-dashboard | Open-source live web monitor | free | Top N active sessions: project, branch, model, context usage, current tool, 3 s refresh | Single user, no metrics/cost history | [GitHub](https://github.com/futin/claude-agents-dashboard) |
| claude-code-otel, ccdashboard | OTel collector + Prometheus/Grafana bundles | free | Ready Grafana dashboards | You run and maintain Grafana | [claude-code-otel](https://github.com/ColeMurray/claude-code-otel), [ccdashboard](https://github.com/NikiforovAll/ccdashboard) |
| Dash0 AI Coding Insights | OTLP + plugin, SaaS | $10/developer/month | Purpose-built session traces | Claude Code and Cursor only; SaaS | [Dash0](https://www.dash0.com/comparisons/claude-code-monitoring-tools) |
| Datadog Agent Console, Grafana Cloud integration, CloudWatch, Dynatrace, Honeycomb, SigNoz, Better Stack | OTLP into general platform | platform pricing | Organisation rollups | SaaS (except SigNoz OSS); no host-to-session correlation; generic | [Dash0](https://www.dash0.com/comparisons/claude-code-monitoring-tools), [SigNoz](https://signoz.io/docs/claude-code-monitoring/) |
| LiteLLM | Gateway with budgets per developer/team | free OSS | Hard budgets, 20+ backends | Extra hop; API keys only, not subscriptions | [Torii](https://www.toriihq.com/articles/five-claude-code-usage-dashboards-and-monitoring-tools) |

### 1.5 Data center / cockpit references

| Product | Pos | Price | Cockpit ideas worth borrowing | Sources |
|---|---|---|---|---|
| Aravolta | AI-era DCIM: branch-circuit, power, BMS, network, assets in one pane; $5.1M raised | not public **(unverified)** | Single pane over heterogeneous telemetry; predictive alerting; automated device discovery in hours | [site](https://www.aravolta.com/), [monitoring](https://www.aravolta.com/dc-management/monitoring), [HostingJournalist](https://hostingjournalist.com/news/aravolta-raises-5-1m-for-ai-era-data-center-control-software) |
| Hyperview | Cloud DCIM | $3/asset/year, minimum 500 assets, from $125/month; public pricing | Interactive power path; capacity utilisation visuals; thermal heat maps; 30-day full resolution + 3-year daily summaries | [pricing](https://hyperviewhq.com/pricing/), [features](https://hyperviewhq.com/tools-and-features/) |
| Sunbird (dcTrack, Power IQ) | Enterprise DCIM | not public **(unverified)** | Colour-coded health map with hover tooltip (latest events, hottest rack, power utilisation); drill-down from site to rack; thermal time-lapse | [3D visualization](https://www.sunbirddcim.com/product/data-center-visualization), [health map](https://www.sunbirddcim.com/screen-shots/dctrack-cabinet-health-floor-map-report) |

---

## 2. Capability matrix

Columns are the strongest comparables. Rating: **B** best-in-class, **G** good, **P** partial, **-** absent.
Ratings are our synthesis of the sources in section 1, not vendor claims.

| Capability (PRODUCT.md domain) | Datadog | Grafana Cloud | Netdata | Beszel | Uptime Kuma | Better Stack | Healthchecks | ccusage / CC monitor | Agent View | Langfuse | Fleetglass v1 target |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 Machine (CPU, RAM, disk, temps, power, SMART) | G | G | **B** (1 s, auto) | **B** (light) | - | P | - | - | - | - | G + honest power (RAPL) |
| 2 Services (systemd, Docker rootless, ports, TLS) | B | G | G | P (systemd 243+, rootless limits) | P (Docker, TLS) | P | - | - | - | - | **B** for rootless + ports |
| 3 Uptime (internal/external probes, status page) | G | G (k6/synthetics) | P | - | **B** (self-hosted) | **B** (SaaS) | P (push) | - | - | - | G |
| 4 Logs (recent errors) | B | B | G | P (200 journal lines) | - | G | - | - | - | - | P (by design) |
| 5 AI agents (sessions, tokens, quotas, cost/project) | P (Agent Console preview, SaaS) | P (prebuilt dashboards) | - | - | - | P (OTel ingest) | - | G (local, quotas) | P (live state, no tokens) | G (traces via hooks) | **B** target |
| 6 Security (updates, logins, sudo, file changes) | G (Cloud SIEM, paid) | P | P | - | - | - | - | - | - | - | G |
| 7 Backups (age, off-site, restore test) | - | - | - | - | - | - | P (job heartbeat) | - | - | - | **B** (nobody does restore tests) |
| 8 Changes (packages, deploys, restarts, merges) | G (events) | P (annotations) | P | - | - | P | - | - | P (PR status) | - | **B** target |
| 9 Alerts (channels, dedup, reminders) | B | G | G | G | **B** (90+ channels) | B (on-call) | G (25+) | - | - | - | G |
| 10 Incidents (timeline, MTTR, availability) | G | P | P | - | P | **B** | - | - | - | - | G |
| 11 Costs (energy, AI subscriptions, API, cloud) | P (cloud cost, paid) | P | - | - | - | - | - | G (AI only) | - | P | **B** target |
| Correlation agent activity x machine x incidents | - | - | - | - | - | - | - | - | - | - | **B** (unique) |
| UX / visual quality | G (dense) | P (DIY) | G | **B** (simple) | G | **B** | G | P (TUI) | G (TUI) | G | **B** target |
| Onboarding (time to first useful screen) | G | P | **B** (zero-config) | **B** | **B** | **B** | **B** | **B** (`npx`) | **B** (built in) | G | B target (one command) |
| Alert quality (noise control, anomaly) | **B** (Watchdog) | G | **B** (ML) | P (thresholds) | P | G | G (grace) | - | - | - | G (rules, honest unknowns) |
| Self-hosted, no data leaves | - | P (OSS) | P (UI closed, cloud optional) | **B** | **B** | - | **B** | **B** | **B** | **B** | **B** |
| Pricing for 1-10 servers | expensive | free tier OK | free 5 / $90 yr homelab | **B** free | **B** free | free 10 monitors | free 20 checks | free | free | free self-host | see section 5 |

---

## 3. "Best of each": one idea per capability that Fleetglass must match or beat

| Capability | Best idea | From | How Fleetglass matches or beats it |
|---|---|---|---|
| Machine | Zero-config auto-detection of everything present (disks, sensors, GPUs) | [Netdata](https://www.netdata.cloud/pricing/) | Auto-detect, but show only what deviates; Beszel-class footprint (~10-15 MB) ([techfuelhq](https://techfuelhq.com/homelab/beszel-vs-netdata-vs-glances-2026/)) |
| Footprint | 10-15 MB agent, one binary | [Beszel](https://github.com/henrygd/beszel) | Keep the ~10 MB target; publish the measured number on the landing page |
| Services | systemd view with recent journal lines per unit | [Beszel](https://beszel.dev/guide/systemd) | Same, plus oneshot/timer units, rootless Docker, and "who restarted it" from the change log |
| Uptime | 20 s probes, keyword/JSON checks, status pages with custom domain | [Uptime Kuma](https://github.com/louislam/uptime-kuma) | Match the probe types that matter; avoid its SQLite trap (tests with 300+ probes) |
| Cron / heartbeats | Cron expression + grace time + start/success/fail signals measuring run time | [Healthchecks.io](https://healthchecks.io/docs/configuring_checks/) | Built-in push endpoint for backups, cron and agent jobs (see 5.1) |
| Logs | Live log panels pinned next to metrics with saved filters | [Railway](https://docs.railway.com/observability) | Recent-error panel next to the chart that moved |
| AI agents: live state | Session roster with state, "needs input", "waiting X m", one-line summary | [Claude Code Agent View](https://code.claude.com/docs/en/agent-view) | Same roster for Claude Code *and* Codex, all users on the host, plus tokens, cost, CPU/RAM per session, history, alerts; summaries from rules, not an LLM |
| AI agents: quota pace | Burn-rate forecast of when the 5-hour block runs out | [Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor), [CodexBar](https://github.com/steipete/CodexBar) | Pace for both 5 h and weekly windows, both vendors, across all sessions on the server, alerting before exhaustion |
| AI agents: cost reports | Daily/monthly/session/project/model reports for 16+ CLIs | [ccusage](https://github.com/ryoppippi/ccusage) | Same aggregates, persisted, per worktree, reconciled with billing APIs when keys exist |
| AI agents: standard ingest | Native OTel export from Claude Code and Codex | [Claude Code docs](https://code.claude.com/docs/en/monitoring-usage), [CloudWatch Codex](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/coding-agents-codex.html) | Built-in OTLP receiver so laptops and other servers can send too |
| Security | Watch permission-mode changes and tool decisions | [Claude Code events](https://code.claude.com/docs/en/monitoring-usage) | Agent-aware security: sudo by agent users, bypass-permission sessions, sensitive files touched |
| Backups | (nobody shows restore tests) | - | Restore-test age as a first-class number |
| Changes | Suspect commits / release markers on errors | [Sentry](https://docs.sentry.io/product/issues/suspect-commits/) | Change markers on every chart; "what changed in the 15 min before" on every incident |
| Alerts | Huge channel catalogue | [Uptime Kuma](https://github.com/louislam/uptime-kuma) | Cover the PRODUCT.md list plus ntfy/Gotify/Pushover/Matrix; generic webhook for the rest |
| Alert quality | Anomaly alerts with a stated rule (5-minute window vs 24 h baseline, > 4 standard deviations, minimum activity) | [Vercel](https://vercel.com/changelog/anomaly-alerts-ga) | Same transparency: every alert shows the rule that fired; grouping like Sentry fingerprints |
| Incidents | Timeline auto-starts on failure; post-mortem draft from timeline | [Better Stack](https://hyperping.com/blog/betterstack-vs-site24x7-vs-hyperping) | Auto timeline enriched with agent activity and changes; export as Markdown |
| Costs | Spend alerts at 50/75/100 % with optional hard stop | [Vercel](https://vercel.com/blog/introducing-spend-management-realtime-usage-alerts-sms-notifications) | Budget alerts per project/agent (read-only: alert, never stop) |
| Cockpit overview | Health map with hover tooltip "latest events, hottest rack, utilisation" and drill-down | [Sunbird](https://www.sunbirddcim.com/product/data-center-visualization) | Fleet tiles: hover gives latest event, hottest resource, active agents; click drills into the server cockpit |
| Retention model | 30 days full resolution + 3 years daily summaries | [Hyperview](https://hyperviewhq.com/pricing/) | Already similar in PRODUCT.md (7 d raw, 90 d hourly, 2 y daily) |
| UX | Cmd+K, single-letter shortcuts, speed as design | [Linear](https://www.925studios.co/blog/linear-design-breakdown-saas-ui-2026) | Command palette ("go to server", "silence alert", "open session"), URL-addressable state |
| Navigation | Organised by job, not by product name; suspicious items surfaced first | [Cloudflare](https://developers.cloudflare.com/changelog/post/2026-01-15-networking-navigation-update/) | "What stands out" first, domains second |
| Pricing clarity | Public, usage-transparent pricing; unlimited seats | [PostHog](https://posthog.com/pricing), [Hyperview](https://hyperviewhq.com/pricing/) | One public price, no per-seat, no metric metering |
| AI access (without LLM in the path) | MCP server so the user's own assistant can query monitoring | [Netdata](https://www.netdata.cloud/features/aiml/mcp/) | Optional read-only MCP endpoint, off by default (compatible with "no LLM in the monitoring path") |

---

## 4. Gaps nobody fills well (Fleetglass can own them)

1. **Session x machine correlation.** No product links an agent session to its process tree (CPU, RSS, I/O, child
   processes such as builds and test runners) on the host. Datadog/Grafana/Dash0 see OTel from the CLI, Beszel/Netdata
   see processes; nobody joins them. Third-party sizing notes that memory, driven by tool calls, is the real
   concurrency bottleneck for agents ([amux](https://amux.io/guides/ai-agent-server-sizing/)) **(claim not independently
   verified)**. Fleetglass can show "session X: 2.1 GB, 3 node processes, triggered the OOM at 14:02".
2. **Server-side quota pacing across vendors.** Quota tools are desktop or terminal (CodexBar macOS, Usage Monitor TUI),
   one user at a time. Claude Code OTel exports no quota metric ([docs](https://code.claude.com/docs/en/monitoring-usage));
   Codex logs it in session JSONL ([tokenatlas #89](https://github.com/Magnus-Gille/tokenatlas/issues/89)). A server
   running several workers under several Unix users needs one pace view and an alert before the 5 h or weekly window
   closes.
3. **Agent health as an alertable state.** Agent View shows "needs input" and "failed" locally, but nothing alerts on
   "session waiting for permission for 30 min", "API retries exhausted", "rate-limited", "idle session holding 1.5 GB",
   "orphan worktree with no process" ([Agent View](https://code.claude.com/docs/en/agent-view),
   [event list](https://code.claude.com/docs/en/monitoring-usage)).
4. **Multi-user hosts.** Agent View is per user ([docs](https://code.claude.com/docs/en/agent-view)); ccusage reads one
   home directory. Real agent servers run several accounts (e.g. root coordinator + `agent` workers).
5. **Cost per project/worktree that mixes subscriptions and API.** Seat plans get no per-token data from Anthropic
   ([Torii](https://www.toriihq.com/articles/five-claude-code-usage-dashboards-and-monitoring-tools)); OpenAI's usage API
   covers API keys, not ChatGPT-plan Codex usage **(unverified for Codex plan usage)**. Fleetglass can show
   "API-equivalent cost" from local logs next to "billed cost" from billing APIs, clearly labelled (honest numbers).
6. **Incidents explained by agent activity and changes.** Better Stack and incident.io build timelines from alerts and
   Slack; none includes "agent session started a build", "agent merged branch", "package upgrade" on the same timeline.
7. **Agent-aware security.** Sudo by agent accounts, sessions in bypass-permission mode
   (`claude_code.permission_mode_changed`), sensitive files modified by an agent user. No monitoring tool frames this.
8. **Fully self-hosted.** Every product with an agent view is SaaS (Datadog, Grafana Cloud, Dash0, CloudWatch) or
   a local single-user tool. Netdata's UI is not open source ([Debian](https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=1045145)).
9. **Backups with restore tests** and **energy cost** are not covered by any of the infra tools reviewed.

---

## 5. Recommendations

### 5.1 Scope changes to PRODUCT.md

**Add**
- **OTLP receiver** (metrics + logs, gRPC/HTTP) in the server, as the primary ingest for Claude Code and Codex, with
  local JSONL parsing as the fallback for subscription users and for quotas. Reason: both CLIs now export OTel natively
  ([Claude](https://code.claude.com/docs/en/monitoring-usage), [Codex](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/coding-agents-codex.html)),
  and file formats are undocumented and change.
- **Session-to-process attribution**: map each agent session (PID, cwd, worktree, Unix user) to its process tree and
  record per-session CPU, RSS, I/O; show it on the session row and on the machine charts.
- **Agent health states and alerts**: needs input > N min, API errors/retries exhausted, rate-limited, quota pace
  forecast to exhaust before reset, stale session, orphan worktree, worktree disk usage.
- **Heartbeat / cron monitoring** (Healthchecks-style push URL with grace time and start/success/fail) in v1, not only
  Healthchecks.io as an alert channel. Backups, cron jobs and scheduled agent runs all need it.
- **Change markers on every chart** and a "what changed before this" block on every incident.
- **Alert hygiene**: maintenance windows, acknowledge/snooze, grouping of related alerts, and a dead-man alert when
  Fleetglass's own agent stops reporting (unknown is never healthy).
- **Command palette + keyboard shortcuts + URL-addressable state** as part of the cockpit milestone.
- **Interoperability**: Prometheus `/metrics` export of Fleetglass's own data (so Grafana users can adopt it without
  leaving Grafana).
- **Optional read-only MCP endpoint** (v1.x, off by default) so the user's own agents can ask Fleetglass about the
  machine. Netdata already markets this ([MCP](https://www.netdata.cloud/features/aiml/mcp/)).

**Cut or defer**
- Optional cloud bills (domain 11): defer to v2; keep energy and AI costs.
- Public status page polish: keep a minimal one in v1 (Uptime Kuma sets the bar at free), no subscriber notifications.
- Generic "git merges" ingestion beyond the repositories where agents run: defer.

**Reorder milestones** (the differentiator must be visible in the first demo, and a monitor without alerts is not usable):
1. Agent + server + storage for machine and services (unchanged).
2. **AI agents** (sessions, process attribution, tokens, quotas, OTLP ingest) — moved up from 3.
3. Cockpit dashboard live, dark/light, command palette — now shows agents from day one.
4. **Alerts and incidents** (incl. heartbeats, agent health alerts, dead-man) — moved up from 5.
5. Multi-server and fleet view — moved down from 4.
6. External probes and public status page.
7. Security (agent-aware), backups (restore tests), changes, costs.
8. One-command install, documentation, landing page.

**Positioning line** (suggested): "The cockpit for servers that run AI coding agents" rather than "zero blind spots
for everything": the second promise invites a feature comparison with Datadog and Netdata that Fleetglass cannot win.

### 5.2 Pricing positioning

| Reference | Price | Source |
|---|---|---|
| Beszel, Uptime Kuma, ccusage | free, unlimited | [Beszel](https://github.com/henrygd/beszel), [Kuma](https://github.com/louislam/uptime-kuma), [ccusage](https://github.com/ryoppippi/ccusage) |
| Netdata | 5 nodes free; $90/year unlimited homelab; $4.50/node/month business | [pricing](https://www.netdata.cloud/pricing/), [homelab](https://www.netdata.cloud/homelab/) |
| Datadog Infrastructure | $15-18/host/month | [last9](https://last9.io/blog/datadog-pricing-all-your-questions-answered/) |
| Dash0 AI Coding Insights | $10/developer/month | [Dash0](https://www.dash0.com/comparisons/claude-code-monitoring-tools) |
| Better Stack | $29/responder/month | [hyperping](https://hyperping.com/blog/betterstack-pricing) |
| Healthchecks.io | $20/month for 100 checks | [foursight](https://foursight.cloud/compare/healthchecks-pricing) |

Recommendations:
- **Free core: up to 3 servers** (not 1), including the full AI agent layer. A 1-server limit loses every homelab user
  to Beszel on day one, and Netdata gives 5. The agent layer is the hook; it should be in the free tier.
- **Paid "Pro"**: either ~€5/server/month (between Netdata Business and Datadog) or a flat ~€15-19/month up to 10
  servers. The flat plan reads better for solo builders, who are the primary target, and avoids per-unit anxiety
  (the main complaint about Datadog/Cronitor). Paid features: fleet beyond 3, long retention, teams/SSO, advanced
  alerting (escalations, on-call), SaaS later.
- **Never meter tokens, sessions or metrics.** Usage-based billing is the most-cited pain in this market
  ([HN](https://news.ycombinator.com/item?id=35837330), [PagerDuty reviews](https://spike.sh/blog/pagerduty-review-for-alerting-2026/)).
- Keep the price public on the landing page (Hyperview/PostHog style).
- €9/server/month is defensible only for the paid fleet tier compared with Datadog, not compared with the free
  self-hosted tools the target users already run.

---

## 6. Risks

| Risk | Detail | Mitigation |
|---|---|---|
| First-party absorption | Anthropic ships Agent View ([docs](https://code.claude.com/docs/en/agent-view)), OTel, Admin/Analytics APIs; OpenAI ships Usage/Costs API. A first-party "usage dashboard" could cover tokens and quotas | Own what vendors will not: multi-vendor, multi-user, host correlation, self-hosted, alerting |
| Big platforms move down-market | Datadog Agent Console, Grafana Cloud integration with a free tier ([Grafana](https://grafana.com/docs/grafana-cloud/observe-and-act/monitor-infrastructure/integrations/integration-reference/integration-claude-code/)) | Stay self-hosted, lighter, cheaper; export Prometheus so they are complements |
| Free self-hosted incumbents | Beszel (MIT, ~26k stars) could add an agent tab; Netdata Homelab $90/year unlimited with AI/MCP | Speed on the agent layer; publish footprint; make the free tier generous |
| Format instability | Local JSONL formats (Claude, Codex) are undocumented and change; Codex `/status` known to show stale limits on first call ([CodexFuse](https://iaplabs.itch.io/codexfuse/devlog/1669898/why-the-first-codex-status-can-show-the-wrong-rate-limits-and-what-to-trust)) | OTLP first, parsers versioned and tested on fixtures, "unknown" shown honestly |
| Terms of service | Reading quota from unofficial endpoints or reusing CLI credentials may conflict with provider terms **(unverified)** | Read only local files and documented APIs/telemetry |
| SQLite scaling | Uptime Kuma's experience past a few hundred monitors ([#6181](https://github.com/louislam/uptime-kuma/issues/6181)) | Downsampling, WAL, write batching, load test early |
| Licensing perception | AGPL core can deter some companies; Netdata's closed UI shows the community punishes "open-washing" ([Debian](https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=1045145)) | Keep the whole free UI under AGPL and say clearly what is paid |
| Market consolidation | Helicone acquired and in maintenance ([chatforest](https://chatforest.com/reviews/helicone-llm-observability-gateway/)), Traceloop acquired by ServiceNow ([aicalculatorpro](https://aicalculatorpro.com/ai-stack/llm-observability/traceloop/)): users fear tool abandonment | Self-hosted, open core, data stays local: a credible "no lock-in" story |
| Name: "Helm" in DevOps | "Helm" is the Kubernetes package manager (CNCF); search and trademark confusion in the same audience **(trademark status unverified)** | Legal search (EUIPO, USPTO, WIPO) in classes 9 and 42 before any purchase |
| Name: Helmcode | Helmcode is an EU AI inference provider that explicitly targets Claude Code / Codex users and is listed in CodexBar ([helmcode.com](https://helmcode.com/), [CodexBar](https://codexbar.app/)); close name, same audience | Highest naming risk found; weigh in the final name decision |
| Name: other uses | "HELMLINE" reported as a HELM Pharma whistleblower channel (search snippet only; the compliance page fetched does not mention it, so **unverified**); `@fleetglass` taken on X ([x.com](https://x.com/fleetglass)); MTG combo name; fleetglass.com registered by a third party (PRODUCT.md) | Include in the legal search; check social handles |

---

## 7. Method and limits

- About 70 searches and 10 page fetches on 2026-10-08. No sign-ups, no logins, no trials.
- Pricing comes from vendor pages where fetched (Netdata, Cronitor, Hyperview, PostHog, New Relic) and otherwise from
  third-party pricing guides; check vendor pages again before publishing any comparison.
- UX observations for Datadog, Better Stack and Statuspage that are not backed by a link are marked unverified.
- The capability matrix ratings are judgement calls based on the sources above.
- No trademark database was queried; all naming notes are preliminary.
