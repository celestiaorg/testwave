BINARY_NAME:=testwave
IMAGE_NAME:=$(BINARY_NAME)-image
REGISTRY:=ttl.sh

build:
	go build -o bin/$(BINARY_NAME) -v ./cmd

clean:
	rm -rf bin/

docker-build:
	docker build -t $(IMAGE_NAME):latest .

docker-push: docker-build
	$(eval IMAGE_ID:=$(shell docker images -q $(IMAGE_NAME):latest))
	$(eval RAND_IMAGE_ID := $(shell uuid))
	docker tag $(IMAGE_ID) $(REGISTRY)/$(RAND_IMAGE_ID):24h
	docker push $(REGISTRY)/$(RAND_IMAGE_ID):24h

.PHONY: build docker-build docker-push clean