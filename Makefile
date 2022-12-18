toolchain: ## Install toolchain for development
	hack/toolchain.sh

verify: licenses ## Run Verifications like helm-lint and govulncheck
	govulncheck ./pkg/...
	golangci-lint run

fmt: ## go fmt the code
	find . -iname "*.go" -exec go fmt {} \;

licenses: ## Verifies dependency licenses
	go mod download
	! go-licenses csv ./... | grep -v -e 'MIT' -e 'Apache-2.0' -e 'BSD-3-Clause' -e 'BSD-2-Clause' -e 'ISC' -e 'MPL-2.0'

help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: verify fmt licenses help toolchain