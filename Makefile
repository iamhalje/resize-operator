IMG ?= resize-operator:latest
CONTAINER_TOOL ?= docker

LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
HELM_DOCS ?= $(LOCALBIN)/helm-docs

CONTROLLER_TOOLS_VERSION ?= v0.20.1

.PHONY: all
all: build

.PHONY: manifests
manifests: $(CONTROLLER_GEN)
	"$(CONTROLLER_GEN)" crd paths="./..." output:crd:artifacts:config=charts/resize-operator/crds

.PHONY: generate
generate: $(CONTROLLER_GEN)
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: helm-docs
helm-docs: $(HELM_DOCS)
	"$(HELM_DOCS)" --chart-search-root=./charts/resize-operator

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: tests
tests:
	go test -race -mod=mod ./...

.PHONY: build
build: manifests generate fmt vet tests helm-docs

.PHONY: docker-build
docker-build:
	$(CONTAINER_TOOL) build -t ${IMG} .

$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN="$(LOCALBIN)" go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

$(HELM_DOCS): $(LOCALBIN)
	GOBIN="$(LOCALBIN)" go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest
