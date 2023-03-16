CONTAINER_SUBSYS := "podman"
NAME := "dwarfbot"
PROJECT := "clcollins"
IMAGE_REGISTRY := "quay.io"

CONTAINER_FILE := "Containerfile"

IMAGE_STRING := $(IMAGE_REGISTRY)/$(PROJECT)/$(NAME)

.PHONY: build
build:
	$(CONTAINER_SUBSYS) build -f $(CONTAINER_FILE) -t $(IMAGE_STRING):latest .

.Phony: test
test:
	go test -v ./...