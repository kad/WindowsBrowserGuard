# GoReleaser Usage Guide

## Prerequisites

1. **Install GoReleaser**:
   ```powershell
   # Using Chocolatey (Windows)
   choco install goreleaser
   
   # Or download from: https://goreleaser.com/install/
   ```

2. **Ensure Git repository is initialized**:
   ```powershell
   git init
   git add .
   git commit -m "Initial commit"
   ```

3. **Set up GitHub repository** (for automated releases):
   - Create a repository on GitHub
   - Add remote: `git remote add origin https://github.com/yourusername/printwatch.git`
   - Push code: `git push -u origin main`

## Local Build and Test

Build snapshot without releasing (for testing):

```powershell
# Test the build configuration
goreleaser build --snapshot --clean

# Build and create archives
goreleaser release --snapshot --clean
```

Built artifacts will be in the `dist/` folder.

## Creating a Release

### Manual Release (Local)

1. **Create and push a tag**:
   ```powershell
   # Create a version tag
   git tag -a v1.0.0 -m "Release version 1.0.0"
   
   # Push the tag
   git push origin v1.0.0
   ```

2. **Run GoReleaser**:
   ```powershell
   # Create GitHub release
   goreleaser release --clean
   ```

   You'll need a GitHub token with `repo` permissions:
   ```powershell
   # Set environment variable
   $env:GITHUB_TOKEN = "your_github_token_here"
   ```

### Automated Release (GitHub Actions)

The included `.github/workflows/release.yml` will automatically:
- Trigger on version tags (e.g., `v1.0.0`)
- Build for Windows AMD64 and ARM64
- Create GitHub release with:
  - Compiled binaries
  - PowerShell scripts
  - README and documentation
  - Checksums
  - Auto-generated changelog

**Usage**:
```powershell
# Commit your changes
git add .
git commit -m "feat: new feature description"

# Create and push a tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions will automatically build and release
```

## Versioning

Follow [Semantic Versioning](https://semver.org/):
- `v1.0.0` - Major.Minor.Patch
- `v1.0.0-beta.1` - Pre-release
- `v1.0.0-rc.1` - Release candidate

## Changelog Generation

GoReleaser automatically generates changelogs from commit messages. Use conventional commits:

- `feat:` - New features
- `fix:` - Bug fixes
- `sec:` - Security updates
- `perf:` - Performance improvements
- `doc:` - Documentation updates
- `chore:` - Maintenance tasks

**Example**:
```powershell
git commit -m "feat: add Firefox extension blocking support"
git commit -m "fix: resolve privilege elevation issue on Windows 11"
git commit -m "sec: validate registry paths before deletion"
```

## Release Process Checklist

1. ✓ Update version in code (if applicable)
2. ✓ Update README.md with new features
3. ✓ Test build locally: `goreleaser build --snapshot --clean`
4. ✓ Commit all changes: `git commit -am "Prepare for vX.Y.Z"`
5. ✓ Create tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
6. ✓ Push tag: `git push origin vX.Y.Z`
7. ✓ Wait for GitHub Actions or run: `goreleaser release --clean`
8. ✓ Verify release on GitHub
9. ✓ Test downloaded artifacts

## Configuration Files

- `.goreleaser.yml` - Main GoReleaser configuration
- `.github/workflows/release.yml` - GitHub Actions workflow
- `go.mod` - Go module dependencies

## Customization

Edit `.goreleaser.yml` to:
- Change build flags and ldflags
- Add/remove architectures
- Modify archive contents
- Configure code signing (optional)
- Set up announcement integrations

## Troubleshooting

**Build fails**:
```powershell
# Clean and retry
goreleaser release --clean --skip-validate

# Check configuration
goreleaser check
```

**GitHub token issues**:
```powershell
# Create token at: https://github.com/settings/tokens
# Required scopes: repo

# Set token
$env:GITHUB_TOKEN = "ghp_your_token_here"
```

**Tag already exists**:
```powershell
# Delete local tag
git tag -d v1.0.0

# Delete remote tag
git push --delete origin v1.0.0

# Create new tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

## Example Release Output

After successful release, users can download:
- `printwatch_1.0.0_Windows_x86_64.zip` (AMD64)
- `printwatch_1.0.0_Windows_arm64.zip` (ARM64)
- `checksums.txt` (SHA256 hashes)

Each archive contains:
- `printwatch.exe`
- `README.md`
- `install-task.ps1`
- `uninstall-task.ps1`
- `start-monitor.ps1`
- `view-logs.ps1`

## Additional Resources

- GoReleaser Documentation: https://goreleaser.com/
- GitHub Actions Documentation: https://docs.github.com/actions
- Conventional Commits: https://www.conventionalcommits.org/
