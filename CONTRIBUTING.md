# Contributing to Railzway

Thank you for your interest in contributing to Railzway!
To maintain a high-quality codebase and a stable release pipeline, we follow a strict **Feature Branch Workflow**.

## 🚀 Branching Strategy

We do **NOT** commit directly to `main`. All changes must go through a Pull Request.

### Branch Types

Please use the following prefixes for your branches:

- **`feat/`**: New features (e.g., `feat/auth-provider-google`)
- **`fix/`**: Bug fixes (e.g., `fix/docker-build-error`)
- **`chore/`**: Maintenance, config tasks, or dependency updates (e.g., `chore/upgrade-go-1.25`)
- **`docs/`**: Documentation updates (e.g., `docs/update-readme`)
- **`refactor/`**: Code restructuring without behavior change (e.g., `refactor/centralize-config`)

### The Golden Rule
> **The `main` branch is always clean, buildable, and deployable.**

## 🛠️ Development Workflow

1.  **Sync**: Always start from up-to-date main.
    ```bash
    git checkout main
    git pull origin main
    ```
2.  **Branch**: Create your feature branch.
    ```bash
    git checkout -b feat/my-awesome-feature
    ```
3.  **Work**: Write code, run tests, and commit locally.
    - We encourage [Conventional Commits](https://www.conventionalcommits.org/):
      - `feat: add google oauth support`
      - `fix: resolve docker build failure`
4.  **Push**: Push your branch to origin.
    ```bash
    git push -u origin feat/my-awesome-feature
    ```
5.  **Pull Request (PR)**:
    - Open a PR on GitHub targetting `main`.
    - Fill in the description.
    - Request review.
6.  **Merge**:
    - Once approved and CI passes, use **Squash and Merge** (preferred) to keep history linear.
    - Delete the branch after merging.

### 📦 Versioning & Release Process

We use [Changesets](https://github.com/changesets/changesets) to manage versions and changelogs.

#### 1. Adding a Changeset
When you make a change that requires a changelog entry (feature, fix, or breaking change), run:

```bash
pnpm changeset
```

- Select the package(s) you modified.
- Select the bump type (major/minor/patch).
- Write a summary of the change.

This creates a markdown file in `.changeset/`. Commit this file along with your code.

#### 2. Release Lifecycle
1.  **Version PR**: A "Version Packages" PR runs automatically on `main`. It consumes all changesets and updates `package.json` versions and `CHANGELOG.md`.
2.  **Tagging**: When the "Version Packages" PR is merged, the system creates Git Tags (e.g., `@railzway/admin@1.0.1`).
3.  **Docker Build**: Git Tags trigger the Docker Build & Publish workflow.

