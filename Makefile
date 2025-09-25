FILES := $(shell git ls-files '*.go')

.PHONY: fmt fmt-check vet test skills run lint ci

fmt:
	@if [ -n "$(FILES)" ]; then gofmt -w $(FILES); fi

fmt-check:
	@if [ -n "$(FILES)" ]; then \
		fmt_out=$$(gofmt -l $(FILES)); \
		if [ -n "$$fmt_out" ]; then \
			echo "The following files need gofmt:"; \
			echo "$$fmt_out"; \
			exit 1; \
		fi; \
	fi

vet:
	go vet ./...

test:
	go test ./...

skills:
	cd skills/examples/timer && mkdir -p build && tinygo build -o build/timer.wasm -target=wasi ./src
	cd skills/examples/smart-home && mkdir -p build && tinygo build -o build/smart-home.wasm -target=wasi ./src
	go run ./cmd/loqa-skill validate --file skills/examples/timer/skill.yaml
	go run ./cmd/loqa-skill validate --file skills/examples/smart-home/skill.yaml

run:
	go run ./cmd/loqad --config ./config/example.yaml

lint: fmt-check vet

ci: lint test skills
