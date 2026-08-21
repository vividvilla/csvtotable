VERSION := $(shell sed -n 's/^  "version": "\(.*\)",/\1/p' npm/package.json)
LDFLAGS := -s -w -X main.version=$(VERSION)
EXE := $(shell go env GOEXE)

.PHONY: frontend build test dist demo

frontend:
	cd table && bun install --frozen-lockfile
	cd table && bun run build

build: frontend
	mkdir -p build
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o build/csvtotable$(EXE) .

test: frontend
	test -z "$$(gofmt -l *.go)"
	go vet ./...
	go test ./...

demo: build
	mkdir -p site
	./build/csvtotable$(EXE) --overwrite \
		--description @demo/description.md \
		--css sample/custom.css \
		--page-size 25 \
		sample/meteorite-landings-1.csv sample/meteorite-landings-2.csv \
		site/index.html

dist: frontend
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/csvtotable$(EXE) .
	python3 scripts/package_release.py --binary dist/csvtotable$(EXE) --version v$(VERSION)
