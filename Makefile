.PHONY: test vet spk check run receiver up down smoke clean

test:
	go test ./...

vet:
	go vet ./...

# Build the Synology package(s). Override with: make spk VERSION=1.1.0 BUILD=2 ARCHES="x86_64 armv8"
spk: vet test
	VERSION=$(or $(VERSION),1.0.0) BUILD=$(or $(BUILD),1) ARCHES="$(or $(ARCHES),x86_64)" sh scripts/build-spk.sh

check:
	sh scripts/check-secrets.sh

# Run locally for development: admin UI at http://127.0.0.1:8580/admin/ without login.
run:
	go run ./cmd/vtunerd serve -dev -data ./data

# Act as a receiver against `make run`.
receiver:
	go run ./cmd/fakereceiver -server 127.0.0.1:8580

# Docker test environment (see docs/testing.md).
up:
	docker compose up --build -d

down:
	docker compose down

smoke:
	docker compose exec -T vtuner fakereceiver -server 127.0.0.1:80 -auto $(if $(filter off,$(VTUNER_DIRECTORY)),-no-directory)

clean:
	rm -rf build dist data
