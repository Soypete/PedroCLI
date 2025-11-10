#!/bin/bash
set -e

echo "Setting up GitHub Actions workflows..."
echo

# Create .github/workflows directory if it doesn't exist
if [ ! -d ".github/workflows" ]; then
    echo "Creating .github/workflows directory..."
    mkdir -p .github/workflows
else
    echo ".github/workflows directory already exists"
fi

# Copy E2E test workflow
if [ -f "docs/examples/github-workflows/e2e-tests.yml" ]; then
    echo "Copying E2E test workflow..."
    cp docs/examples/github-workflows/e2e-tests.yml .github/workflows/
    echo "✓ Copied e2e-tests.yml"
else
    echo "⚠ Warning: docs/examples/github-workflows/e2e-tests.yml not found"
fi

# Copy CI workflow
if [ -f "docs/examples/github-workflows/ci.yml" ]; then
    echo "Copying CI workflow..."
    cp docs/examples/github-workflows/ci.yml .github/workflows/
    echo "✓ Copied ci.yml"
else
    echo "⚠ Warning: docs/examples/github-workflows/ci.yml not found"
fi

echo
echo "✅ GitHub Actions workflows have been set up!"
echo
echo "The following workflows are now active:"
echo "  • .github/workflows/e2e-tests.yml - Runs on PR ready for review + manual trigger"
echo "  • .github/workflows/ci.yml - Runs unit tests on every push/PR"
echo
echo "Next steps:"
echo "  1. Review the workflow files in .github/workflows/"
echo "  2. Commit and push:"
echo "     git add .github/workflows/"
echo "     git commit -m 'Add GitHub Actions workflows for CI and E2E tests'"
echo "     git push"
echo
