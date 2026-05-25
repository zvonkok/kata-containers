# QEMU Machine-Centric Architecture: Migration Plan

> Operational plan for migrating the QEMU command-line generator in `runtime-rs` to the machine-centric architecture described in issue [#12187](https://github.com/kata-containers/kata-containers/issues/12187). This document captures the **how** of the migration; the **what** lives in the issue body, and the **why** lives in the issue body and the linked discussion threads.

## 0. Pointers

- Design issue (the contract): [kata-containers/kata-containers#12187](https://github.com/kata-containers/kata-containers/issues/12187)
- Migration tracker (per-phase status, updated as PRs land): see the **Migration Tracker** section at the top of #12187's body.
- Roadmap umbrella (NUMA / Grace / cache-coherent): [kata-containers/kata-containers#12125](https://github.com/kata-containers/kata-containers/issues/12125)
- Reference command lines: §9 of issue #12187 (Grace, vCMDQ, vEGM)
- Related VRA document: [docs/design/kata-vra.md](../../../docs/design/kata-vra.md)

## 1. Strategy

**Strangler over big-bang. No long-lived feature branch.** Each PR is small, single-concern, and either preserves byte-identical QEMU CLI output (refactor) or extends it in a documented way (feature). The §9 reference CLIs are the contract — every PR is judged against them.

The constraint at the top of every decision: **the transition must be consumable and reviewable**. Anything that violates that constraint is wrong, no matter how clean the destination looks.

## 2. Phasing

| Phase | Goal | Approx. PRs | Behaviour change? |
| ----- | ---- | ----------- | ----------------- |
| 0 | Test harness, fixtures, dead-code data types | 3 | None |
| 1 | `PlatformProbe` + `Host` constructed but unused | 1 | None |
| 2 | Strangle bus resolution one device at a time | ~9 | None (golden tests unchanged) |
| 3 | Lift `Objects` registry (memory backend, thread context first) | 2 | None |
| 4 | Multi-RC PCIe (`pxb-pcie`, per-RC SMMUv3, `vfio-pci-nohotplug`, `iommufd`); §9.1 passes | 3 | New host only (`Gb300Nvl72`) |
| 5 | vCMDQ + vEGM; §9.2 and §9.3 pass | 2 | New host only |
| 6 | Dead-code removal, feature-flag cleanup | 1 | None |

Phase 0 is itself split so it is reviewable:

- **0a** — golden-test harness + one trivial fixture
- **0b** — the three §9 fixtures + smoke test that they parse
- **0c** — empty data types (`PciTopology`, `Objects`, `NumaTopology`, …) with unit tests on pure helpers (`PciTopology::resolve`, `Platform::add_vegm_node`, etc.). Not wired into `QemuCmdLine` yet.

## 3. Per-PR Rules

1. **Hard cap ~300 lines diff.** Bigger → split.
2. **Single concern: refactor or feature, never both.** Golden tests unchanged → refactor. Golden tests change → feature, and the fixture diff explains the change.
3. **Title is `[qemu-archi N/M] component: short description`.** The phase prefix lets reviewers sort and group at a glance; the `component: description` part follows Kata's commit convention (the component is usually `runtime-rs` here, sometimes `runtime-rs/qemu` or `runtime-rs/docs` when the change is scoped to a subdir). Example: `[qemu-archi 2/9] runtime-rs: resolve bus via PciTopology in DeviceVirtioBlk`.
4. **Description follows the template** in the Appendix and explains **WHY and HOW**, not WHAT. The diff already shows the WHAT. A reviewer needs the motivation (what constraint or invariant this PR serves) and the approach (why this shape, what alternatives were ruled out). If the description only restates what the diff shows, it has failed.
5. **No mixed-arch changes.** A PR that changes x86 behavior must keep aarch64 byte-identical, and vice versa. Both golden tests pass.

## 4. Invariants

For reviewers picking up a PR cold. If a PR violates one of these, it is wrong:

- `iommufd0` is emitted at most once. It is declared once in `Objects` and referenced everywhere via the typed `IommufdRef`.
- `NumaNodeKind::InitiatorTarget` nodes carry no CPUs and no `memdev`. They exist only as targets for `AcpiPciNodeLink` entries.
- Every `AcpiPciNodeLink::*.pci_dev` must resolve to a device id present in the emitted command line.
- Every `vfio-pci-nohotplug.bus` must be a root port id defined by an earlier `pcie-root-port` switch.
- Emission order is `Objects` → `PciTopology` → ordinary devices. A device referencing an id defined later is a bug.
- For vEGM, `MemoryBackend::File.path` and `AcpiPciNodeLink::EgmMemory.id` share a basename convention: `/dev/egm{N}` ↔ `egm{N}`. Always go through `Platform::add_vegm_node` to avoid drift.
- IOMMU lives on `PciRootComplex`, not on `Machine`. Multiple SMMU instances are normal (Grace emits four).
- `apply_host_defaults` is the **only** place where `Host` is matched. Devices and the cmdline builder never branch on host flavour directly.
- `#[cfg(target_arch)]` should appear in no device's `qemu_params()`. If a PR adds one, it is going the wrong way.
- **CC mode forces non-reconfigurable VFIO.** If `runtime.confidential_guest != Disabled`, all VFIO passthrough devices must be `vfio-pci-nohotplug` (the VMM cannot be reconfigured at runtime once a CC guest is up; hotplug events break attestation and memory-mapping integrity). Compiler-checked: route all VFIO passthrough through `Platform::add_vfio_passthrough(...)`, which selects the variant based on CC mode. Plain `DeviceVfioPci::new()` must not be called directly for passthrough. If a PR uses it, that is a bug.
- **User config overrides host defaults; `validate()` is the final step.** `Platform::from_config_and_probe()` runs in three phases: (1) `apply_host_defaults()` populates from the detected `Host`; (2) `apply_user_overrides(cfg)` overlays user TOML on top; (3) `validate()` rejects impossible combinations (SEV-SNP on aarch64, `cmdqv` on a non-Arm SMMU, etc.). PRs that add new configurable fields must update all three phases consistently.

## 5. Naming Conventions (id schema)

| QEMU object | id format | Example |
| ----------- | --------- | ------- |
| `pxb-pcie` | `pcie.{bus_nr}` | `pcie.1` |
| `pcie-root-port` | `pcie.port{bus_nr}` | `pcie.port1` |
| `arm-smmuv3` / SMMU instance | `smmuv3.{ordinal}` | `smmuv3.1` |
| `iommufd` backend | `iommufd0` (single instance) | `iommufd0` |
| memory backend | `m{node_id}` | `m0` |
| `acpi-generic-initiator` | `gi{ordinal}` | `gi7` |
| `acpi-egm-memory` + device file | `egm{node_id}` / `/dev/egm{node_id}` | `egm4` / `/dev/egm4` |

## 6. Test Infrastructure

- **Golden CLI tests** live under `src/runtime-rs/crates/hypervisor/src/qemu/tests/fixtures/`. One fixture per reference CLI: `grace.cli`, `vcmdq.cli`, `vegm.cli`. String comparison only — no QEMU process invocation needed.
- **Test harness** loads a fixture, builds a `Platform` from a known config + a forced `Host`, emits the CLI, and asserts equality.
- **Force Host for testing** via `KATA_QEMU_FORCE_HOST_FLAVOR=Gb300Nvl72` (or similar). Lets CI exercise the Grace code path on non-Grace hardware.
- **Both architectures.** Run x86_64 and aarch64 unit tests. If Kata's CI lacks aarch64 unit-test coverage, treat that as a Phase-0 sub-task.

## 7. Pre-PR Self-Checklist

Before requesting review:

- [ ] Diff under 300 lines.
- [ ] Single concern (refactor xor feature).
- [ ] Golden tests pass on x86 and aarch64.
- [ ] PR title carries the phase prefix.
- [ ] Description template is filled.
- [ ] Linked in the tracking checklist on issue #12187.

## 8. Reviewer-Friendly Practices

- **Consistent reviewer set across the sequence.** Two or three people see every PR in the chain. They build up shared context; the author does not re-explain the architecture each time.
- @pmores should be a default reviewer. His critiques have the most leverage when applied to the same shape repeatedly and can see the strangler converging.
- **Stacked PRs** (or sequential base branches) for the Phase-2 device-by-device chain. Each PR reviewable independently, but lands in a clear order.
- **North-star draft PR** sits open as read-only context, demonstrating §9.1 produced end-to-end from a single `apply_host_defaults` arm. Not for review; just for "this is where this is going."
- **In-tree design doc (this file) plus issue #12187** are the two artifacts a reviewer needs to ramp up. Everything else should be derivable.

## 9. Risks

- **Concurrent #12125 PRs (#12161, #12162, #12163, …).** Rebase strangler PRs onto their merges, not vice versa. Phase 2's device migrations may best be combined with #12162's bus changes (one PR per device covering both).
- **aarch64 CI coverage.** Cross-arch divergence is the most likely regression source. Golden tests must cover both.
- **Legacy `memory_backend: Option<String>` field** on what is currently `Machine` (will become `BaseMachine` after the rename in §3). Phase 3 needs a compatibility shim: keep the field, populate it from `Objects.memory_backends[0].id` during the transition.
- **Emission-ordering retrofit is painful.** Build the `Objects` → `PciTopology` → devices order from Phase 0, even while `Objects` is still empty.
- **Scope creep into RuntimeProfile / config-driven overlays.** Section 10 of the original proposal was removed for a reason. Don't reintroduce named profiles by accident.

## 10. Follow-ups (out of scope for this design)

These are explicitly *not* part of #12187 but the design should leave clean integration points for them.

- **`vra_platform=...` config knob.** A TOML setting (e.g. `vra_platform = "Gb300Nvl72"`) that bypasses `PlatformProbe::detect()` and forces a specific `Host` value. Useful for (a) running Kata on non-standard hardware that should impersonate a known platform, (b) testing the GB300 / DGX code paths without that exact hardware, (c) operators that prefer explicit declaration over autodetection. Hooks into the same `from_config_and_probe` flow as the env-var override in §6; separate PR after Phase 4.
- **`pcie-lib` host PCIe topology introspection.** A probe that reads the host's physical PCIe tree (which GPUs are peer-to-peer, NUMA locality of physical devices, root-port topology) and feeds it into `apply_host_defaults`. Orthogonal to `PlatformProbe::detect()` (which only identifies *which* host); this informs *what to build* once the host is identified. Separate from this design.
- **vEGM §9.3 fixture sizes.** The reference CLI as supplied has `-m size=16G` paired with `memory-backend-file size=32G`. The other §9 examples (Grace, vCMDQ) have matching sizes, so this is treated as a one-off typo in the source. The §9.3 fixture uses 16G consistently. Ping the original author if confirmation matters, but it does not block.

## 11. First Concrete Steps

In order, before any device migration begins:

1. Write this document (done if you are reading it).
2. Add the **Migration Tracker** section to #12187's body, with one `[ ]` per planned PR (done — see top of #12187).
3. Open Phase 0a (test harness) as the first PR, and tick the corresponding box in the tracker once it merges.
4. Open the north-star draft PR — empty scaffold pointing at the `grace.cli` fixture as the goal — to make the destination visible without committing to a merge timeline.

Only after these exist does Phase 0b/0c/1/2 work begin.

## Appendix: PR Title and Description Template

**Title format:** `[qemu-archi N/M] component: short description`

- `N/M` is the position in the sequence (e.g. `2/9` for the second of nine Phase-2 device migrations).
- `component` follows Kata's commit convention. Typically `runtime-rs`; use `runtime-rs/qemu` or `runtime-rs/docs` when the change is scoped to a subdirectory.
- Keep the short description focused on the *change*, not the *destination*. "resolve bus via PciTopology in DeviceVirtioBlk" — yes. "step toward Grace support" — no, that's what the PR body is for.

Example: `[qemu-archi 2/9] runtime-rs: resolve bus via PciTopology in DeviceVirtioBlk`

**Description template:**

> **Write WHY and HOW, not WHAT.** The diff is the WHAT — reviewers can read it. The description has to carry the *motivation* (the constraint, invariant, or §-of-#12187 this PR serves) and the *approach* (why this shape, what alternatives were ruled out). "Structural change" is HOW; "Semantic change" is its consequence; neither should restate the diff.

```markdown
**Phase / step:** 2/9
**Issue:** #12187 (see §5)
**Tracker:** #12187 (Migration Tracker section — tick the box for this PR on merge)

**Why this PR (motivation):**
- (one or two sentences: the constraint or invariant this serves)

**How (approach taken):**
- (one or two sentences: the shape of the change, alternatives ruled out)

**Structural change:**
- (one sentence)

**Semantic change:**
- None.
- (or: fixture diff explained in `tests/fixtures/<file>.cli`)

**Golden tests:** passing unchanged / updated as noted above
**Architectures verified:** x86_64, aarch64
**Depends on:** #PR-NNNN (if any)
```
