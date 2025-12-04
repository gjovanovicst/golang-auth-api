# Nancy Vulnerability Scanner Setup

## Overview

Nancy is an optional vulnerability scanner from Sonatype that checks your Go dependencies against the OSS Index for known security vulnerabilities. It's integrated into the CI/CD pipeline but requires authentication to function.

## Current Configuration

Nancy is configured with `continue-on-error: true` in the GitHub Actions workflow, meaning:
- ✅ The CI pipeline will **not fail** if Nancy authentication is missing
- ✅ Gosec security scanner still runs and provides security scanning
- ⚠️ Nancy will show a warning but won't block your builds

## Why Nancy Requires Authentication

Nancy connects to the [Sonatype OSS Index](https://ossindex.sonatype.org/) which requires a free account to prevent abuse and rate limiting.

## How to Enable Nancy (Optional)

If you want to enable full vulnerability scanning with Nancy, follow these steps:

### 1. Create a Free OSS Index Account

1. Go to https://ossindex.sonatype.org/
2. Click "Sign Up" and create a free account
3. Verify your email address

### 2. Get Your API Token

1. Log in to OSS Index
2. Go to your account settings
3. Generate an API token
4. Copy your username and token

### 3. Add GitHub Secrets

Add the following secrets to your GitHub repository:

1. Go to your repository on GitHub
2. Navigate to **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Add these two secrets:
   - Name: `NANCY_USERNAME`, Value: your OSS Index username
   - Name: `NANCY_TOKEN`, Value: your OSS Index API token

### 4. Update the Workflow (Optional)

If you added the secrets, you can update `.github/workflows/ci.yml` to use them:

```yaml
      - name: Run Nancy vulnerability scanner
        if: ${{ !env.ACT }}
        continue-on-error: true
        env:
          NANCY_USERNAME: ${{ secrets.NANCY_USERNAME }}
          NANCY_TOKEN: ${{ secrets.NANCY_TOKEN }}
        run: |
          go install github.com/sonatype-nexus-community/nancy@latest
          if [ -n "$NANCY_TOKEN" ]; then
            go list -json -deps ./... | nancy sleuth --username "$NANCY_USERNAME" --token "$NANCY_TOKEN"
          else
            go list -json -deps ./... | nancy sleuth || echo "⚠️ Nancy scan skipped - requires OSS Index authentication."
          fi
```

## Local Usage

To run Nancy locally:

### Without Authentication (Limited)
```bash
go install github.com/sonatype-nexus-community/nancy@latest
go list -json -deps ./... | nancy sleuth
```

### With Authentication (Recommended)
```bash
# Set environment variables
export NANCY_USERNAME="your_username"
export NANCY_TOKEN="your_token"

# Run Nancy
go list -json -deps ./... | nancy sleuth --username "$NANCY_USERNAME" --token "$NANCY_TOKEN"
```

Or add to your `.env` file (don't commit this):
```bash
NANCY_USERNAME=your_username
NANCY_TOKEN=your_token
```

## Alternative: Use Make Command

The Makefile includes a vulnerability scan command:

```bash
make vulnerability-scan
```

This will attempt to run Nancy. If you have credentials configured, it will use them.

## What Nancy Checks

Nancy scans your Go dependencies (`go.mod` and transitive dependencies) against the OSS Index database for:
- Known CVEs (Common Vulnerabilities and Exposures)
- Security advisories
- Vulnerability severity scores
- Affected version ranges
- Remediation recommendations

## Current Security Coverage

Even without Nancy, your CI pipeline includes:
- ✅ **Gosec** - Static analysis security scanner for Go code
- ✅ **Go Module Checksums** - Ensures dependency integrity
- ✅ **Unit Tests** - Including security-focused tests
- ✅ **Code Review** - Manual security review process

Nancy adds an additional layer by checking for known vulnerabilities in dependencies.

## Troubleshooting

### "401 Unauthorized" Error
This means Nancy couldn't authenticate with OSS Index. Either:
- You haven't set up credentials (this is fine - it won't break CI)
- Your credentials are incorrect
- Your API token has expired

### Rate Limiting
Without authentication, OSS Index has strict rate limits. If you hit them:
- Wait a few minutes and try again
- Consider setting up authentication (free and unlimited)

### Nancy Installation Issues
If Nancy fails to install:
```bash
# Clear Go module cache
go clean -modcache

# Reinstall
go install github.com/sonatype-nexus-community/nancy@latest
```

## Recommendations

### For Open Source Projects
- Authentication is optional but recommended
- Use GitHub secrets to protect credentials
- Document in your CONTRIBUTING.md if you require Nancy to pass

### For Private/Enterprise Projects
- **Strongly recommended** to set up authentication
- Consider making Nancy a required check (remove `continue-on-error`)
- Regular vulnerability scanning is a security best practice

### For Personal Projects
- Authentication is optional
- Gosec provides good security coverage without it
- Enable Nancy if you want comprehensive dependency scanning

## Resources

- [Nancy GitHub Repository](https://github.com/sonatype-nexus-community/nancy)
- [OSS Index](https://ossindex.sonatype.org/)
- [Sonatype Documentation](https://ossindex.sonatype.org/doc/rest)
- [GitHub Actions Secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets)

## Summary

Nancy is **optional** and **non-blocking** in this project's CI pipeline. The security-scan job will pass regardless of Nancy's status. If you want full vulnerability scanning, follow the setup steps above. Otherwise, Gosec provides excellent security scanning for your code.

