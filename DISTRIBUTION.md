# Distribution Guide

How to release keel and distribute it via Homebrew.

---

## One-time setup

### 1. Create the tap repository

On GitHub, create a new **public** repository named exactly `homebrew-keel` under your org/account.

```
github.com/locustave/homebrew-keel
```

The name must start with `homebrew-` — this is how Homebrew resolves `brew tap locustave/keel`.

Initialise it with the scaffold in this repo:

```bash
cd homebrew/
git init
git remote add origin git@github.com:locustave/homebrew-keel.git
git add .
git commit -m "initial tap scaffold"
git push -u origin main
```

### 2. Create a Personal Access Token

Go to **GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens**.

Create a token with:
- **Resource owner**: locustave (or your account)
- **Repository access**: Only `homebrew-keel`
- **Permissions → Contents**: Read and Write

Copy the token.

### 3. Add the secret to the keel repo

Go to **keel repo → Settings → Secrets and variables → Actions → New repository secret**.

- Name: `HOMEBREW_TAP_TOKEN`
- Value: the token from step 2

### 4. Replace placeholder org name

Search for `locustave` across:
- `homebrew/Formula/keel.rb`
- `homebrew/README.md`
- `.github/workflows/release.yml` (already uses `${{ github.repository }}` — nothing to change)

---

## Releasing a new version

```bash
# 1. Make sure all tests pass
make test

# 2. Tag the release (semver)
git tag v1.0.0
git push origin v1.0.0
```

The release workflow (`.github/workflows/release.yml`) then automatically:

1. Runs `go test ./...`
2. Builds cross-platform binaries via `make release`
3. Creates a GitHub Release with the binaries attached
4. Computes the SHA256 of the auto-generated source tarball
5. Commits an updated `Formula/keel.rb` to `homebrew-keel`

The formula update lands in the tap within ~30 seconds of pushing the tag. Users already on keel get the update on the next `brew upgrade keel`.

---

## User install (after setup)

```bash
brew tap locustave/keel
brew trust locustave/keel
brew install keel
keel --help
```

---

## Install from HEAD (latest main)

```bash
brew install locustave/keel/keel --HEAD
```

This builds directly from the `main` branch. Useful for testing unreleased changes.

---

## Build targets

| Command | What it does |
|---------|-------------|
| `make build` | Build `bin/keel` for local dev |
| `make test` | Run all Go tests |
| `make release` | Build all 4 platform binaries into `dist/` |
| `make install` | Build and copy to `/usr/local/bin/keel` |
| `make clean` | Remove `bin/` and `dist/` |

---

## Platform targets

`make release` produces four binaries in `dist/`:

| File | Platform |
|------|---------|
| `keel-darwin-arm64` | macOS Apple Silicon |
| `keel-darwin-amd64` | macOS Intel |
| `keel-linux-amd64` | Linux x86-64 |
| `keel-linux-arm64` | Linux ARM64 |

---

## Troubleshooting

**Formula SHA256 mismatch**
GitHub generates the source tarball on-demand. If the SHA256 step races with tarball generation, re-run the workflow manually from the Actions tab.

**Tap update fails with 403**
The `HOMEBREW_TAP_TOKEN` has expired or lacks Contents write permission on `homebrew-keel`. Regenerate and update the secret.

**`brew install` builds old version**
Run `brew update` first — this fetches the latest formula from the tap before installing.

**Testing the formula locally before release**

```bash
# Point brew at the local formula file directly
brew install --build-from-source homebrew/Formula/keel.rb
```
