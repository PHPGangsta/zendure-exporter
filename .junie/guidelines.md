### Project Overview
`zendure-exporter` is a Prometheus exporter for Zendure SolarFlow systems. It fetches real-time data from the Zendure cloud API and exposes it in a format that Prometheus can scrape.

### How to Run Tests
To run all tests locally, use the standard Go test command:
```bash
go test -v ./...
```
This will execute unit tests for all packages (client, collector, config) and integration tests.

### Linting
To maintain code quality, we use `golangci-lint`. You should run it locally before pushing:
```bash
golangci-lint run
```

If you don't have it installed, you can install it via:
- **Snap (Linux)**: `sudo snap install golangci-lint`
- **Homebrew (macOS/Linux)**: `brew install golangci-lint`
- **Go**: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

### Local Development & Docker
While the project includes a `Dockerfile` for deployment, running `docker build -t zendure-exporter:ci .` locally might fail on some systems due to file permissions if the local environment has different UID/GID settings than the container's build context. 
The recommended way to test locally is:
1.  **Directly via Go**: `go build ./cmd/zendure-exporter`
2.  **Using Docker Compose**: `docker-compose up --build` (which handles the environment setup more gracefully).

### Versioning & Git Tags
We use semantic versioning (e.g., `v1.0.0`). The version is injected at build time using Git tags.
**Important**: Before pushing to GitHub, ensure you have tagged your commit if you want to trigger a release build.

1.  **Create a tag**: `git tag v1.0.0`
2.  **Push the tag**: `git push origin v1.0.0`

### GitHub Release (Optional but Recommended)
To make it look official on your repository's homepage:
1.  Go to your repository on GitHub.
2.  On the right side, click on **"Releases"** -> **"Create a new release"**.
3.  Click **"Choose a tag"** and select `v1.0.0`.
4.  Give it a title (e.g., `Initial Release v1.0.0`).
5.  Click **"Generate release notes"** (GitHub will automatically list your commits).
6.  Click **"Publish release"**.
