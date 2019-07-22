#!/bin/bash

set -eEuo pipefail

# retry because resolving the central config might race
retry_default gen_envoy_bootstrap s1 19000 primary
retry_default gen_envoy_bootstrap s2 19001 primary
retry_default gen_envoy_bootstrap s3 19002 primary