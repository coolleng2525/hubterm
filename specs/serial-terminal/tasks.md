# Tasks: 多人共享串口终端

> Spec: [spec.md](./spec.md)
> Plan: [plan.md](./plan.md)

## Conventions

- Tasks are ordered. Don't skip ahead unless a dependency is independently met.
- Each task has an observable done signal.
- Mark `[x]` completed, `[~]` in progress, `[ ]` not started.

## Task list

### Foundation

- [x] **T1: Add serial protocol and persistent configuration model**
  - **Does:** Defines serial parameters, commands, defaults, session protocol, and migration-safe persistence.
  - **Files:** `internal/proto/types.go`, `internal/center/model/models.go`, `go.mod`, `go.sum`, model/handler tests.
  - **Done when:** Model migration and protocol validation tests pass.
  - **Depends on:** nothing.

- [x] **T2: Build the Agent serial session manager**
  - **Does:** Opens one physical port per session, streams bytes, writes input, reports status, and closes safely.
  - **Files:** `internal/agent/serialsession/manager.go`, `internal/agent/serialsession/manager_test.go`.
  - **Done when:** Fake-port tests cover open, duplicate rejection, read/write, failure, close, and close-all.
  - **Depends on:** T1.

- [x] **T3: Wire serial commands into the Agent**
  - **Does:** Handles `serial_start`, `serial_close`, input routing, session reporting, and connector disconnect cleanup.
  - **Files:** `cmd/agent/main.go`, `internal/agent/connector/connector.go`, reporter and connector tests.
  - **Done when:** Agent tests pass and darwin/linux/windows agent binaries cross-compile.
  - **Depends on:** T2.

### Center implementation

- [x] **T4: Add acknowledged Agent command execution**
  - **Does:** Lets Center wait for serial open/close success or a bounded timeout before returning API success.
  - **Files:** `internal/center/handler/agent_ws.go`, protocol tests.
  - **Done when:** Success, Agent error, disconnect, and timeout tests pass without leaking pending requests.
  - **Depends on:** T1.

- [x] **T5: Add serial configuration and connection APIs**
  - **Does:** Validates/saves parameters, creates or reuses a unique serial session, and rolls back failed opens.
  - **Files:** `internal/center/handler/serial_terminal.go`, `serial_port.go`, `node.go`, `session.go`, `cmd/center/main.go`, tests.
  - **Done when:** Handler tests cover permissions, defaults, invalid parameters, concurrent connects, reuse, failure, and release.
  - **Depends on:** T3, T4.

- [x] **T6: Enforce shared-session participant roles**
  - **Does:** Tracks browser participants, assigns one master, blocks observer input, transfers control, kicks participants, and closes idle serial sessions.
  - **Files:** `internal/center/handler/terminal_participants.go`, `ws.go`, participant/WebSocket tests.
  - **Done when:** Multi-client tests prove one master, observer read-only behavior, transfer, kick, and last-participant cleanup.
  - **Depends on:** T5.

### Frontend implementation

- [x] **T7: Add serial row actions and parameter editor**
  - **Does:** Adds Connect, New-tab Connect, and Edit with loading, empty, error, offline, and busy states.
  - **Files:** `web/src/api/index.js`, `web/src/views/NodeDetail.vue`.
  - **Done when:** Production build succeeds and each state has an observable UI path.
  - **Depends on:** T5.

- [x] **T8: Add master/observer controls to Shared Terminal**
  - **Does:** Shows connection parameters, participant roles, control actions, and prevents observer input in the UI.
  - **Files:** `web/src/views/SharedTerminal.vue`.
  - **Done when:** Production build succeeds and protocol-driven role changes update without reload.
  - **Depends on:** T6, T7.

### Edge cases and verification

- [x] **T9: Run integration, race, and cross-platform verification**
  - **Does:** Exercises full Go tests, race-sensitive packages, frontend build, shell checks, and three Agent targets.
  - **Files:** test files and task notes only unless failures require fixes.
  - **Done when:** All automated checks pass and no debug instrumentation remains.
  - **Depends on:** T1–T8.

- [~] **T10: Verify with a real USB serial port**
  - **Does:** Connects to one reported `/dev/cu.usbserial-*` device, opens a second observer, transfers control, and disconnects cleanly.
  - **Files:** task notes only unless a defect is found.
  - **Done when:** Hardware behavior is confirmed or the exact unverified steps are documented for the user.
  - **Depends on:** T9.

- [x] **T11: Run the production-grade checklist**
  - **Does:** Reviews UX states, inputs, concurrency, permissions, accessibility, responsiveness, observability, and performance.
  - **Files:** implementation and task notes; fixes where needed.
  - **Done when:** Applicable checklist items are verified and skipped items are documented.
  - **Depends on:** T9.

- [x] **T12: Add persistent serial port aliases**
  - **Does:** Saves an optional operator alias and shows it in the port list and shared terminal without hiding the real device path.
  - **Files:** serial configuration model/handler/tests, `NodeDetail.vue`, `SharedTerminal.vue`, spec and plan.
  - **Done when:** Migration, API, frontend build, and alias fallback behavior pass verification.
  - **Depends on:** T5, T7, T8.

## Notes log

- **2026-07-22:** Spec and plan confirmed. Implementation started with T1.
- **2026-07-22:** T1 completed; protocol and migration tests pass.
- **2026-07-22:** T2 completed; serial manager tests and race detector pass.
- **2026-07-22:** T3 completed; connector tests pass and Agent cross-compiles for macOS, Linux, and Windows.
- **2026-07-22:** T4 completed; acknowledged commands pass success, error, timeout, disconnect, and race tests.
- **2026-07-22:** T5 completed; serial defaults, config persistence, connection reuse, and disconnect tests pass.
- **2026-07-22:** T6 completed; participant roles, transfer, kick, idle cleanup, and WebSocket race tests pass.
- **2026-07-22:** T7–T8 completed; row actions, parameter editor, participant controls, and production frontend build pass.
- **2026-07-22:** T9 completed; full tests, race detector, vet, shell syntax, production build, and three Agent targets pass.
- **2026-07-22:** T10 real `/dev/cu.usbserial-A1UNDCUI` open/busy/close/release passed. Two-browser visual verification remains because the in-app browser was unavailable.
- **2026-07-22:** T11 completed; pre-merge frontend review fixed popup blocking, observer input state, parameter visibility, and macOS duplicate ports.
- **2026-07-23:** T12 started after real-device testing showed raw device paths are difficult to distinguish reliably.
- **2026-07-23:** T12 completed and deployed locally; migration preserved existing settings, `A1WNL8P4` persisted as `brown-serial`, and Center served the new frontend bundle.
