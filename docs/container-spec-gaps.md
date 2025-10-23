Here’s what’s missing / needs tightening to make this production-ready. I’ve grouped it so you can turn these into PR checklists.

# Architecture & Security

* **Threat model + isolation guarantees.** Define what the container protects against (file exfil, network egress, host DoS). Document approved data flows.
* **No Docker-socket hardening.** `-v /var/run/docker.sock` is a huge footgun. Options:

  * Run the relay **rootless** and use rootless Docker.
  * Put a **Docker API proxy** with an allowlist (only create/attach/stop on labeled containers).
  * Run the relay inside a VM (firecracker/Kata) or migrate to **containerd**/**Kubernetes** with a narrow CRI.
* **User namespaces & capabilities.** Add:

  * `userns-remap` support (or Podman rootless).
  * AppArmor/SELinux profile (custom or docker-default), `seccomp` profile pinned (not just `"default"` string).
  * `no-new-privileges` always on; drop **ALL** caps and add a small allowlist if needed.
* **Filesystem hardening.**

  * If `ReadOnlyRootFS=true`, mount needed writeable paths as `tmpfs` (`/tmp`, `/run`, `/home/agent`, npm cache).
  * **Workspace ownership:** you create host dirs 0755 as root; container runs UID 1000. Chown/chgrp or set userns so the agent can write.
  * **Size limits:** enforce per-workspace **quota** (project quotas or overlay size), prevent disk fills.
  * **SELinux** hints (`:z` / `:Z`) for bind mounts on enforcing hosts.
* **Network policy.**

  * If `network_mode:none`, how does the agent call model APIs? If you need egress, define **egress allowlist**, DNS policy, and proxy support (`HTTP(S)_PROXY`, CA bundle).
  * If dev uses bridge, define **runtime toggle** + audit.
* **Secrets management.**

  * You don’t show how `ANTHROPIC_API_KEY` or others enter the container. Add a per-session **env injection** path with explicit allowlist, never writing to disk; support **Docker secrets**/tmpfs files.

# Reliability & Ops

* **Container reuse/pooling policy.** Cold start for every session is expensive. Define a pool (warm containers) + **TTL**, **max sessions** per node, and backpressure (queue/reject).
* **Startup/health lifecycle.**

  * Healthcheck currently `pgrep node`; entrypoint is `npx @zed-industries/claude-code-acp`. Make healthcheck query the agent’s **/health** (or a tiny TCP check) instead of process name.
  * Add **liveness/readiness** semantics the relay respects before attaching.
* **Resource limits completeness.**

  * Add `PidsLimit`, `MemorySwap` (=Memory), `OOMKillDisable=false`, ulimits (`nofile`, `nproc`), CPU set/weight, I/O throttles (blkio).
  * Per-user/org **quotas** and **rate limits** to prevent noisy neighbor.
* **Orphan cleanup on boot.** On relay start, **reconcile**: detect leftover `label=acp.relay=true` and reap them. Also prune orphan workspaces.
* **Metrics & tracing.**

  * Emit **SLIs**: session startup latency, attach success rate, OOMs, container lifetime, bytes in/out, disk used, queue wait.
  * OpenTelemetry traces: `sessionId` → container lifecycle spans.
  * Structured logs with labels (`sessionId`, `containerId`, `imageDigest`).
* **Supply chain.**

  * Pin images by **digest**, not tag; generate **SBOM**; run **vuln scans**; sign with **cosign**; record provenance.
  * Build with `docker buildx` multi-arch; reproducible build flags.
* **Platform coverage.**

  * Document Mac/Windows (Docker Desktop socket path), rootless Linux, cgroup v2 specifics.
  * Optional abstraction layer to support **containerd/CRI** in future (don’t hard-wire to Moby types everywhere).

# API/Code Issues (bugs & footguns)

* **Stream demuxing.** With `Tty:false`, Docker **multiplexes stdout/stderr**. You assign both to `attachResp.Reader`. Use `stdcopy.StdCopy` (or set `Tty:true` and accept no stderr separation).
* **Stdin handling.** Use `attachResp.Conn` but support `CloseWrite()` to signal EOF; guard concurrent writes; expose backpressure.
* **Missing imports / compile errors.**

  * `types.go`: `fmt` used but not imported.
  * `manager.go`: uses `os`, `strings` but not imported in the snippet.
* **CPU quota math.** If `CPULimit` is 0 or <0, you’ll set `CPUQuota=0`. Validate and default. Consider `NanoCPUs` instead (clearer).
* **Memory parsing edge cases.** Support lowercase suffixes, spaces, and errors on invalid values; add `Ti/Gi` if you’ll ever run big boxes.
* **Log driver options** validation; set sane defaults across Docker versions.
* **Container command.** You rely on image `ENTRYPOINT`. If the agent requires flags (e.g., stdio mode), add **configurable Cmd/Args**.
* **Networking names.** Validate `NetworkMode` strings; when `host`, document Linux-only.
* **Workspace cleanup race.** If `AutoRemove=true` and you remove workspace on session end, make sure the agent is detached and file handles closed to avoid EBUSY.
* **Monitor loop → StopContainer recursion.** `monitorContainer` calls `StopContainer()` which locks `mu`. It’s fine now, but be wary of re-entrancy if you later add callbacks inside the lock.

# Config & Docs

* **Config versioning & migration.** Add `configVersion`, migration notes, and validation with helpful error messages.
* **Per-env overrides for secrets, egress, and resources.** Examples for dev/staging/prod with explicit differences.
* **Audit logging.** Who created a session, from where, what image digest, what mounts, start/stop times.
* **Privacy/PII stance.** What goes into logs; redaction policy.
* **SLOs/alerts.** Page on high OOM rate, long startup times, container create failures, disk almost full, socket permission errors.

# Testing gaps

* **Unit tests compile.** Your sample tests reference `client` without import and use `alpine:latest` with no long-running process—container exits immediately. Fix by running `["sh","-c","sleep 60"]` or a tiny echo server.
* **Demux tests.** Add tests that verify stderr vs stdout separation and stdin EOF handling.
* **Chaos & limits.** Tests for OOM (exit 137), CPU throttling, disk-full, pids exhaustion, slow pulls.
* **Concurrency.** Spin up N sessions, assert quotas, and teardown correctness.
* **CI runners.** Document how tests run in CI (privileged runner, rootless Docker, or DinD).

# Image/Dockerfile polish

* Pin exact versions (node:20-slim digest; npm package pinned, not `latest`).
* Add `npm ci --omit=dev` or cache optimization; drop editors (`vim`, `nano`) from prod image.
* If `ReadOnlyRootFS`, configure npm cache to writable tmpfs.
* Consider a **distroless**/Wolfi base for smaller attack surface.

# Feature gaps (quality of life)

* **Command/Env templating per session.** Support dynamic envs (API keys, org IDs), cwd, initial files.
* **File ingest/export.** Document how uploads/downloads map to `/workspace` (size limits, timeouts).
* **Session heartbeats** and idle shutdown timers.
* **Versioned images per agent type** if you’ll support multiple agent binaries soon.

---

## Quick code nits (copy-paste ready)

* Use stdcopy:

```go
r, w := io.Pipe()
go func() {
  defer w.Close()
  _, _ = stdcopy.StdCopy(w, w, attachResp.Reader) // writes stdout+stderr, or split with two pipes if you want separation
}()
cnt.Stdout = r
cnt.Stdin = attachResp.Conn
type closeWriter interface{ CloseWrite() error }
if cw, ok := attachResp.Conn.(closeWriter); ok { /* keep for EOF */ }
```

* Set more limits:

```go
hostConfig.Resources.PidsLimit = ptr
hostConfig.Resources.MemorySwap = memoryLimit
hostConfig.Ulimits = []*units.Ulimit{
  {Name: "nofile", Soft: 65536, Hard: 65536},
  {Name: "nproc",  Soft: 4096,  Hard: 4096},
}
```

* Tmpfs when RO root:

```go
hostConfig.Tmpfs = map[string]string{"/tmp":"rw,noexec,nosuid,nodev,size=256m", "/run":"rw,nosuid,nodev,size=64m"}
```

* Warm pool idea: maintain `[]*Container` ready with label `acp.pool=warm`, pop on session create.

If you want, I can turn these into PRs: (1) security hardening, (2) stream handling & tests, (3) secrets/egress, (4) ops metrics & cleanup.

