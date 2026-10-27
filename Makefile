.DEFAULT_GOAL := dev
.PHONY: dev install test test-db test-e2e build
install:
	cd web && bun install --frozen-lockfile
dev:
	cd web && bun run dev
test:
	cd web && bun run content:check && bun run typecheck && bun run lint && bun run test
test-db:
	python3 scripts/test-database.py
test-e2e:
	cd web && bun run test:e2e
build:
	cd web && bun run build
