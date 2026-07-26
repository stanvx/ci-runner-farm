module github.com/stanvx/ci-runner-farm/daemon

// github.com/actions/scaleset declares go 1.25.3, so anything older refuses to
// build it and `go mod tidy` raises this line back to 1.25.3 if it is lowered.
// There is deliberately no `toolchain` line: go strips one that equals the go
// directive and then fails every build with "updates to go.mod needed", so the
// exact compiler is pinned by the SHA-pinned setup-go step in CI instead.
go 1.25.3

require (
	github.com/actions/scaleset v0.4.0
	github.com/golang-jwt/jwt/v4 v4.5.2
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
)
