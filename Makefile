.PHONY: build frontend test run

frontend:
	cd web && pnpm install --frozen-lockfile && pnpm build
	node scripts/sync-web.mjs

build: frontend
	go build -trimpath -o bin/westeros ./cmd/server

test:
	node --test scripts/lib/*.test.mjs
	go test -race -count=1 ./...
	go vet ./...
	cd web && pnpm test && pnpm format:check

run: build
	./bin/westeros
