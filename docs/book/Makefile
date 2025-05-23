# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Directories.
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
TOOLS_DIR := $(realpath ../../hack/tools)
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR := bin
MDBOOK_INSTALL := $(realpath ../../scripts/install-mdbook.sh)

export PATH := $(TOOLS_BIN_DIR):$(PATH)

# Binaries.
TABULATE := $(TOOLS_BIN_DIR)/mdbook-tabulate
$(TABULATE): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/mdbook-tabulate ./mdbook/tabulate

EMBED := $(TOOLS_BIN_DIR)/mdbook-embed
$(EMBED): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/mdbook-embed ./mdbook/embed

RELEASELINK := $(TOOLS_BIN_DIR)/mdbook-releaselink
$(RELEASELINK): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/mdbook-releaselink ./mdbook/releaselink

# Only set MDBOOK if it's not set as an environment variable
MDBOOK ?= $(TOOLS_BIN_DIR)/mdbook
$(MDBOOK):
	$(MDBOOK_INSTALL) v0.4.11 $(TOOLS_BIN_DIR)

RUNTIME_OPENAPI_GEN_BIN := runtime-openapi-gen
RUNTIME_OPENAPI_GEN := $(abspath $(TOOLS_BIN_DIR)/$(RUNTIME_OPENAPI_GEN_BIN))

runtimesdk-yaml:
	$(MAKE) -C $(ROOT_DIR)/../../ $(RUNTIME_OPENAPI_GEN_BIN)
	$(RUNTIME_OPENAPI_GEN) --version book --output-file ./src/tasks/experimental-features/runtime-sdk/runtime-sdk-openapi.yaml

BOOK_DEPS := runtimesdk-yaml

.PHONY: serve
serve: $(MDBOOK) $(TABULATE) $(EMBED) $(RELEASELINK) runtimesdk-yaml
	$(MDBOOK) serve -p 3001

.PHONY: build
build: $(MDBOOK) $(TABULATE) $(EMBED) $(RELEASELINK) runtimesdk-yaml
	$(MDBOOK) build
