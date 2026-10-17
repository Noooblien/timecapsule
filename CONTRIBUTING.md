# Contributing to TimeCapsule

Thank you for contributing to TimeCapsule! ⏳

## Development Workflow

1. Fork and clone the repository.
2. Run tests to verify clean environment:
   ```bash
   go test -v ./tests
   ```
3. Make your changes in a feature branch:
   ```bash
   git checkout -b feat/your-feature
   ```
4. Ensure all tests pass and code is formatted:
   ```bash
   go fmt ./...
   go test -v -race ./tests
   ```
5. Submit a pull request.
