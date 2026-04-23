# Shared conventions for the plan

> Create when ≥ 2 phases share a rule/command. Delete unused sections — no placeholders.

## Pre-commit validation block

<Single fenced block, copy-pasteable. Referenced from every non-doc task's final step. Docs-only tasks skip.>

```
<shell command(s)>
```

## Hygiene checks

<Greps/sweeps every phase must keep green (e.g. `rg -i <banned-token> <paths>` empty). Referenced from each phase's Validation conditions.>

## Cross-phase dependencies

<Items introduced in phase X and consumed in phase Y: symbols (types, interfaces, helpers), concepts, or files/paths. Prevents "defined later, used earlier" drift.>

| Item | Introduced | Consumed |
|------|-----------|----------|
| `<FQCN, concept, or file/path>` | Phase X.N | Phase Y.M, Z.K |

## Final phase checks

<Checks run at the end of *every* phase, not only the last. Referenced from each phase's Validation conditions. Distinct from `INDEX.md#final-verification` (runs once after the last phase).>
