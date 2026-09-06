# 24 Game — build & run
# web (Vue) -> embedded into the Go binary -> single static executable

SHELL := /bin/bash

.PHONY: help dev build-web build test lint clean docker-build docker-run

help:
	@echo "make dev          - run web dev server (vite, hot reload, proxied API) + go server"
	@echo "make build-web    - build Vue SPA into web/dist"
	@echo "make build        - copy dist into server embed dir and build single binary"
	@echo "make test         - go tests + web core tests"
	@echo "make lint         - go vet + gofmt check"
	@echo "make clean        - remove build artifacts"
	@echo "make docker-build - build container image"

# ---------- development ----------

dev:
	@echo "starting go API on :8080 and vite dev server on :5173 ..."
	cd server && go run ./cmd/server &
	cd web && npm run dev

# ---------- build ----------

build-web:
	cd web && npm ci --no-audit --no-fund 2>/dev/null || npm install --no-audit --no-fund
	cd web && npm run build

build: build-web
	rm -rf server/internal/webui/dist
	cp -r web/dist server/internal/webui/dist
	cd server && go build -trimpath -ldflags "-w -s" -o game24-server ./cmd/server
	@echo "built server/game24-server (SPA embedded)"

# ---------- tests ----------

test:
	cd server && go test ./...
	cd web && npm test

lint:
	cd server && go vet ./... && test -z "$$(gofmt -l . | tee /dev/stderr)"

clean:
	rm -rf web/dist server/internal/webui/dist server/game24-server

# ---------- docker ----------

docker-build:
	docker build -t game24 .

docker-run:
	docker run --rm -p 8080:8080 -v game24-data:/data game24
