<div align="center">
  <h1>CLD Changesets</h1>
  <!-- <a><img src="https://github.com/smartcontractkit/cld-changesets/actions/workflows/push-main.yml/badge.svg" /></a> -->
  <br/>
  <br/>
</div>

This repository contains reusable common CLD Changesets such as MCMS, Datastore, JD and more which are used in CLD to perform deployments.

### `cre`

Package [`cre`](./cre) holds CRE-related changesets and operations. 

**`cre_workflow_deploy`:** resolves pre-built workflow WASM, workflow config, and **`project.yaml`** (GitHub release, URL, or local), writes **`workflow.yaml`** and **`context.yaml`**, and runs `cre workflow deploy` via the framework [`operations`](https://pkg.go.dev/github.com/smartcontractkit/chainlink-deployments-framework/operations) API.

- **Changeset:** `cre/changesets.CREWorkflowDeployChangeset`
- **Input:** `cre.CREWorkflowDeployInput` — pipeline YAML supplies `project` ([`cre/artifacts.ConfigSource`](https://pkg.go.dev/github.com/smartcontractkit/chainlink-deployments-framework/cre/artifacts#ConfigSource)), `deploymentRegistry`, and the workflow bundle (binary, config, name). RPCs and `cre-cli` settings live in the user-provided `project.yaml`.
- **Operation:** `cre/operations.CREWorkflowDeployOp`

Domain resolvers can be pass-through; pipeline YAML must include `donFamily`, `project`, and the workflow bundle fields.

#### Package layout

| Package | Purpose |
|---------|---------|
| `cre` | Shared types (`CREWorkflowDeployInput`, `CREDeployDeps`) |
| `cre/changesets` | CLD changesets (e.g. `CREWorkflowDeployChangeset`) |
| `cre/operations` | Idempotent CLD operations invoked by changesets |

## Usage

```bash
go get github.com/smartcontractkit/cld-changesets
```

## Contributing

For instructions on how to contribute to `cld-changesets` and the release process,
see [CONTRIBUTING.md](https://github.com/smartcontractkit/cld-changesets/blob/main/CONTRIBUTING.md)

## Releasing

For instructions on how to release `cld-changesets`,
see [RELEASE.md](https://github.com/smartcontractkit/cld-changesets/blob/main/RELEASE.md)
