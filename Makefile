.PHONY: build run test clean docker-build docker-run

APP_NAME := cks-terminal-mgmt
IMAGE := registry.fullstack.pw/library/$(APP_NAME)

build:
	go build -o $(APP_NAME) ./cmd/server

run: build
	SSH_KEY_PATH=$(HOME)/.ssh/id_ed25519 SSH_USER=suporte ./$(APP_NAME)

test:
	go test ./...

clean:
	rm -f $(APP_NAME)

docker-build:
	docker build -t $(IMAGE):dev .

docker-run: docker-build
	docker run -p 8080:8080 \
		-v $(HOME)/.ssh/id_ed25519:/home/appuser/.ssh/id_ed25519:ro \
		-e SSH_KEY_PATH=/home/appuser/.ssh/id_ed25519 \
		-e SSH_USER=suporte \
		$(IMAGE):dev

fmt:
	go fmt ./...

vet:
	go vet ./...
