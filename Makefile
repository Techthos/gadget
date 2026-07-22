.PHONY: assets test test-go test-ui typecheck verify-dist vet

# Build the TS/CSS bundle into internal/assets/dist (committed, go:embed'd).
assets:
	npm --prefix ui ci
	npm --prefix ui run build

test: test-go test-ui

test-go:
	go test ./...

test-ui:
	npm --prefix ui run test

typecheck:
	npm --prefix ui run typecheck

vet:
	go vet ./...

# Fails if committed dist does not match a fresh build of ui/ sources.
verify-dist: assets
	git diff --exit-code internal/assets/dist
