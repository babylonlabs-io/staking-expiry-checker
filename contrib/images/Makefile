DOCKER = $(shell which docker)

.PHONY: staking-expiry-checker staking-expiry-checker-rmi

staking-expiry-checker:
	$(DOCKER) build --tag babylonlabs-io/staking-expiry-checker -f staking-expiry-checker/Dockerfile \
				$(shell git rev-parse --show-toplevel)

staking-expiry-checker-rmi:
	$(DOCKER) rmi babylonlabs-io/staking-expiry-checker 2>/dev/null; true
