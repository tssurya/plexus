CONTROLLER_GEN ?= go run sigs.k8s.io/controller-tools/cmd/controller-gen
GOLANGCI_LINT ?= golangci-lint

IMG ?= plexus-controller:latest

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Code Generation

.PHONY: generate
generate: ## Generate deepcopy methods and CRD manifests
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd
	$(CONTROLLER_GEN) rbac:roleName=plexus-controller paths="./internal/controller/..." output:rbac:artifacts:config=config/rbac

.PHONY: manifests
manifests: ## Generate CRD manifests only
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd

.PHONY: verify-codegen
verify-codegen: generate ## Verify generated files are up to date
	@if [ -n "$$(git status --porcelain api/ config/crd/ config/rbac/)" ]; then \
		echo "ERROR: generated files are out of date. Run 'make generate' and commit the result."; \
		git diff api/ config/crd/ config/rbac/; \
		exit 1; \
	fi

##@ Development

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: fmt vet ## Run linters
	$(GOLANGCI_LINT) run ./...

.PHONY: lint-api
lint-api: ## Run kube-api-linter on API types
	@if [ ! -f bin/golangci-lint-kube-api-linter ]; then \
		echo "Building kube-api-linter..."; \
		$(GOLANGCI_LINT) custom --custom-gcl-config hack/lint/.custom-gcl.yml; \
	fi
	bin/golangci-lint-kube-api-linter run --config hack/lint/.golangci-api.yml ./api/...

.PHONY: test
test: ## Run unit tests
	go test ./... -coverprofile cover.out -race

##@ Build

.PHONY: build
build: ## Build controller and CLI binaries
	go build -o bin/plexus-controller ./cmd/controller/
	go build -o bin/kubectl-plexus ./cmd/kubectl-plexus/
	ln -sf kubectl-plexus bin/plexus

.PHONY: run
run: ## Run controller locally against the configured cluster
	go run ./cmd/controller/

.PHONY: docker-build
docker-build: ## Build container image
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push container image
	docker push $(IMG)

##@ Deployment

.PHONY: install
install: manifests ## Install CRDs into the configured cluster
	kubectl apply -f config/crd/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the configured cluster
	kubectl delete -f config/crd/

.PHONY: deploy
deploy: manifests ## Deploy controller to the configured cluster
	kubectl apply -f config/manager/
	kubectl apply -f config/rbac/

.PHONY: undeploy
undeploy: ## Undeploy controller from the configured cluster
	kubectl delete -f config/manager/
	kubectl delete -f config/rbac/

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/ cover.out
