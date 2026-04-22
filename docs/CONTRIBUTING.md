# Contributing to Infra-Composer CLI

**Welcome!** We appreciate your interest in contributing. This guide helps you get started.

---

## Code of Conduct

We are committed to providing a welcoming and inclusive environment. All contributors are expected to:
- Be respectful and constructive
- Welcome diverse perspectives
- Focus on feedback, not personal criticism
- Help create a harassment-free space

---

## Getting Started

### 1. Fork & Clone
```bash
# Fork on GitHub, then:
git clone https://github.com/YOUR_USERNAME/infra-composer-cli.git
cd infra-composer-cli

# Add upstream remote
git remote add upstream https://github.com/tiziano093/infra-composer-cli.git
```

### 2. Set Up Development Environment
```bash
# Install Go 1.21+
go version

# Install dependencies
go mod download

# Run tests locally
make test

# Build
make build

# Verify it works
./bin/infra-composer --version
```

### 3. Create Feature Branch
```bash
git checkout develop
git pull upstream develop
git checkout -b feature/your-feature-name
```

---

## Development Workflow

### Before You Start
1. **Check Issues:** Look for open issues or discussions about your idea
2. **Create Issue:** If none exists, open an issue describing:
   - What problem you're solving
   - Proposed solution
   - Why it's important
3. **Wait for Feedback:** Maintainers will provide guidance

### While You Work
1. **Follow Guidelines:** Read DEVELOPMENT_GUIDELINES.md
2. **Write Tests First:** Use TDD approach
   - Write failing test
   - Implement feature
   - Tests pass
3. **Keep Commits Clean:**
   - One logical change per commit
   - Descriptive commit messages
   - Reference issue: `Fixes #123` in description
4. **Run Checks Locally:**
   ```bash
   make fmt          # Format code
   make lint         # Run linter
   make test         # Run tests
   ```

### Before Creating PR
```bash
# Sync with upstream
git fetch upstream
git rebase upstream/develop

# Push to your fork
git push origin feature/your-feature-name

# Create Pull Request on GitHub
```

---

## Pull Request Process

### PR Checklist
- [ ] Branch is up to date with `develop`
- [ ] Tests pass: `make test`
- [ ] Linting passes: `make lint`
- [ ] Code formatted: `make fmt`
- [ ] Test coverage >80% for new code
- [ ] Documentation updated
- [ ] Commit messages are clear

### PR Description Template
```markdown
## Description
Brief description of what this PR does.

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation
- [ ] Refactoring
- [ ] Performance improvement

## Related Issues
Fixes #(issue number)

## Testing
- [ ] Added unit tests
- [ ] Added integration tests
- [ ] Tested locally on multiple platforms

## Documentation
- [ ] Updated README/docs
- [ ] Added code comments for complex logic
- [ ] Updated CHANGELOG

## Checklist
- [ ] Code follows project style
- [ ] No unnecessary dependencies added
- [ ] No breaking changes (or discussed)
- [ ] Performance impact assessed
```

### Review Process
- At least 1 maintainer review required
- Changes requested must be addressed
- Approved PRs are merged by maintainers
- Merge strategy: Squash + merge for clean history

---

## Types of Contributions

### Bug Reports
```markdown
## Description
Clear description of the bug.

## Steps to Reproduce
1. Run `infra-composer catalog build --provider aws`
2. Observe error message

## Expected Behavior
Should generate schema in <30 seconds.

## Actual Behavior
Fails with timeout after 60 seconds.

## Environment
- OS: macOS 13.4
- CLI Version: v1.0.0
- Go Version: 1.21
```

### Feature Requests
```markdown
## Feature Description
What would you like to add?

## Problem It Solves
Why is this important?

## Proposed Solution
How should it work?

## Alternatives Considered
Other approaches?

## Additional Context
Examples, references, etc.
```

### Documentation Improvements
- Fix typos
- Clarify unclear sections
- Add examples
- Update outdated references

**Process:**
1. Make edits in docs/
2. Submit PR with `docs/` prefix
3. No testing required for docs-only changes
4. Automated checks verify Markdown validity

### Performance Improvements
- Profile before and after
- Include benchmarks in PR
- Explain optimization strategy
- Ensure no correctness regressions

---

## Commit Message Guidelines

### Format
```
type(scope): subject line (50 chars max)

Optional detailed description. Wrap at 72 characters.
Explain what and why, not how.

Fixes #123
```

### Types
- `feat` — New feature
- `fix` — Bug fix
- `docs` — Documentation
- `style` — Formatting/whitespace
- `refactor` — Code restructuring
- `perf` — Performance improvement
- `test` — Test additions/updates
- `chore` — Build, dependencies, tooling

### Examples
```
feat(catalog): add fuzzy search for module names

Implement fuzzy matching in search command using Levenshtein distance.
Allows users to find modules with partial/misspelled names.

Fixes #42

---

fix(compose): handle circular dependencies correctly

Previously ignored transitive cycles. Now detects and reports
with helpful suggestion to break cycle.

Fixes #88

---

docs(quickstart): update installation instructions

Add Homebrew installation method and Windows WSL notes.
```

---

## Testing Requirements

### For New Features
- ✅ Unit tests for all exported functions
- ✅ Integration tests for workflows
- ✅ 80%+ code coverage
- ✅ Test fixtures included

### For Bug Fixes
- ✅ Regression test (fails before fix, passes after)
- ✅ Unit test for the specific bug
- ✅ Integration test if affects workflow

### Test File Locations
```
Feature in internal/catalog/searcher.go
  ├─ Unit test: test/unit/catalog_searcher_test.go
  └─ Integration test: test/integration/search_test.go

Feature in internal/commands/compose.go
  ├─ Unit test: test/unit/commands_compose_test.go
  └─ Integration test: test/integration/compose_test.go
```

### Running Tests
```bash
# All tests
make test

# Specific package
go test ./internal/catalog/... -v

# Specific test
go test -run TestSchemaValidate ./test/unit/...

# With coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Code Review Expectations

### What Reviewers Look For
- ✅ Correctness (does it work?)
- ✅ Testing (adequate coverage?)
- ✅ Design (follows architecture?)
- ✅ Performance (any regressions?)
- ✅ Documentation (is it clear?)
- ✅ Style (follows guidelines?)

### Constructive Feedback
We aim for respectful, helpful reviews:
- ❌ "This is wrong"
- ✅ "This could fail if X happens. Consider..."

- ❌ "Use a better approach"
- ✅ "Consider using pattern X because it provides..."

- ❌ "This doesn't work"
- ✅ "I tested this and found Y fails. Here's how to reproduce..."

### Responding to Feedback
- Be open to suggestions
- Ask for clarification if unclear
- Don't hesitate to discuss approaches
- All feedback is about code, not you

---

## Release Process

**Releases are managed by maintainers.** Contributors get automatic attribution via:
- GitHub contributor graph
- Commit history
- CHANGELOG entry (if applicable)

---

## Project Structure Overview

```
infra-composer-cli/
├── cmd/               ← Entry point
├── internal/          ← Business logic (not exported)
├── pkg/               ← Public API
├── test/              ← Tests & fixtures
├── docs/              ← Documentation
├── examples/          ← Usage examples
├── scripts/           ← Build automation
└── .github/           ← GitHub config (workflows, templates)
```

See ARCHITECTURE.md for detailed structure.

---

## Common Questions

**Q: Can I work on multiple issues?**  
A: Yes, but focus on one at a time. Create separate feature branches.

**Q: How long until my PR is reviewed?**  
A: Usually 2-3 business days. We review in order.

**Q: What if my PR gets rejected?**  
A: Feedback is provided. You can address it or close the PR. No hard feelings!

**Q: Can I take over an abandoned PR?**  
A: Ask the maintainers. We'll help coordinate.

**Q: How do I become a maintainer?**  
A: Regular, high-quality contributions over time. Reach out if interested.

---

## Useful Resources

- **DEVELOPMENT_GUIDELINES.md** — Coding standards
- **ARCHITECTURE.md** — System design
- **TESTING_STRATEGY.md** — Testing approach
- **Go Code Review Comments** — https://github.com/golang/go/wiki/CodeReviewComments
- **GitHub Collaboration Guide** — https://docs.github.com/en/github/collaborating-with-pull-requests

---

## Need Help?

- **Questions:** Open a GitHub Discussion
- **Bug Report:** Open an Issue
- **Feature Idea:** Open an Issue with `[Feature Request]` prefix
- **Urgent:** Email project owner (check README)

---

## Thank You!

Thank you for considering contributing to Infra-Composer CLI. We're excited to collaborate with you! 🎉

---

**Last Updated:** 2026-04-22  
