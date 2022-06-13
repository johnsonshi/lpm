.PHONY: build-cli
build-cli:
	go build -v -o ./bin/lpm ./cmd/cli

.PHONY: check-registry-env
check-registry-env:
ifndef REGISTRY
	@echo "[!] REGISTRY is not defined."
	@echo "[!] Format: myregistry.myserver.io/ (trailing slash required)."
	@echo "[!] Example: REGISTRY=\"myregistry.myserver.io/\""
	exit 1
endif

.PHONY: build-example-dockerfiles
build-example-dockerfiles: check-registry-env
	docker buildx build \
		--push \
		-f ./examples/dockerfiles/python-base.dockerfile \
		-t $(REGISTRY)python-base:latest \
		./examples/dockerfiles
	docker buildx build \
		--push \
		-f ./examples/dockerfiles/python-layered-simple.dockerfile \
		-t $(REGISTRY)python-layered-simple:latest \
		./examples/dockerfiles

.PHONY: pull-example-image-manifests
pull-example-image-manifests: check-registry-env
	mkdir -p ./examples/manifests/subject-image-manifests
	docker pull --quiet $(REGISTRY)python-base:latest \
		&& docker manifest inspect $(REGISTRY)python-base:latest \
		> ./examples/manifests/subject-image-manifests/python-base.json
	echo "[*] image manifest saved to: ./examples/manifests/subject-image-manifests/python-base.json"
	docker pull --quiet $(REGISTRY)python-layered-simple:latest \
		&& docker manifest inspect $(REGISTRY)python-layered-simple:latest \
		> ./examples/manifests/subject-image-manifests/python-layered-simple.json
	echo "[*] image manifest saved to: ./examples/manifests/subject-image-manifests/python-layered-simple.json"

.PHONY: build-all-examples
build-all-examples: build-example-dockerfiles pull-example-image-manifests
