DOCKER ?= docker
IMAGE ?= pdf-engine-stage01
ARTIFACTS ?= $(CURDIR)/artifacts/stage-01
SECURITY_OPT ?= --security-opt no-new-privileges
RUN_ID ?= $(shell date -u +%Y%m%dT%H%M%SZ)

.PHONY: build qa probe integration audit unit fixtures
build:
	$(DOCKER) build --target probe -t $(IMAGE):probe .
qa:
	$(DOCKER) build --target qa -t $(IMAGE):qa .
probe: build
	mkdir -p $(ARTIFACTS)
	$(DOCKER) run --rm --network none --read-only --cap-drop ALL $(SECURITY_OPT) --memory 2g --cpus 2 --pids-limit 128 --user $$(id -u):$$(id -g) --tmpfs /tmp:rw,noexec,nosuid,size=256m -v $(ARTIFACTS):/work/jobs $(IMAGE):probe -qa=false -out /work/jobs/$(RUN_ID)-probe
integration: qa
	mkdir -p $(ARTIFACTS)
	$(DOCKER) run --rm --network none --read-only --cap-drop ALL $(SECURITY_OPT) --memory 2g --cpus 2 --pids-limit 128 --user $$(id -u):$$(id -g) --tmpfs /tmp:rw,noexec,nosuid,size=256m -v $(ARTIFACTS):/work/jobs $(IMAGE):qa -out /work/jobs/$(RUN_ID)-qa
audit:
	./scripts/check-test-dependencies.sh
unit: audit
	go test -v ./...
fixtures: qa
	$(DOCKER) run --rm --network none --user $$(id -u):$$(id -g) -v $(CURDIR)/testdata/pdf:/fixtures --entrypoint python3 $(IMAGE):qa /fixtures/generate.py

.PHONY: runtime e2e
runtime:
	$(DOCKER) build --target runtime -t pdf-compression:stage06 .
e2e:
	./scripts/check-test-dependencies.sh
	$(DOCKER) build --target api-test -t pdf-compression:stage06-api-test .
	$(DOCKER) run --rm --init --network none --read-only --cap-drop ALL --memory 2g --cpus 2 --pids-limit 128 --tmpfs /tmp:rw,exec,nosuid,size=1g pdf-compression:stage06-api-test

.PHONY: quality-build quality-check quality-load quality-tests
QUALITY_IMAGE ?= pdf-compression:stage07-quality
QUALITY_ARTIFACTS ?= $(CURDIR)/artifacts/stage-07
QUALITY_RUN ?= quality-$(RUN_ID)
QUALITY_PYTHON ?= python3
quality-build:
	$(DOCKER) build --target quality-test -t $(QUALITY_IMAGE) .
quality-check: quality-build
	mkdir -p $(QUALITY_ARTIFACTS)
	$(DOCKER) run --rm --init --network none --read-only --cap-drop ALL $(SECURITY_OPT) --memory 2g --cpus 2 --pids-limit 128 --user $$(id -u):$$(id -g) --tmpfs /tmp:rw,noexec,nosuid,size=256m -v $(CURDIR):/src:ro -v $(QUALITY_ARTIFACTS):/evidence $(QUALITY_IMAGE) --mode quality --out /evidence/$(QUALITY_RUN)
quality-load: quality-build
	$(DOCKER) run --rm --init --network none --read-only --cap-drop ALL $(SECURITY_OPT) --memory 2g --cpus 2 --pids-limit 128 --user $$(id -u):$$(id -g) --tmpfs /tmp:rw,noexec,nosuid,size=256m --tmpfs /work/jobs/runtime:rw,noexec,nosuid,size=1200m,mode=1777 -v $(CURDIR):/src:ro -v $(QUALITY_ARTIFACTS):/evidence $(QUALITY_IMAGE) --mode load --out /evidence
quality-tests: quality-build audit
	$(DOCKER) run --rm --init --network none --read-only --cap-drop ALL $(SECURITY_OPT) --memory 2g --cpus 2 --pids-limit 128 --user $$(id -u):$$(id -g) --tmpfs /tmp:rw,exec,nosuid,size=1g --entrypoint go -e PDF_LOAD_FIXTURES=/evidence/load-fixtures -v $(CURDIR):/src:ro -v $(QUALITY_ARTIFACTS):/evidence:ro $(QUALITY_IMAGE) test -v -count=1 ./tests/integration ./tests/e2e

.PHONY: quality-fixtures
quality-fixtures:
	$(QUALITY_PYTHON) testdata/pdf/generate_quality.py
