#!/bin/sh

export AAS_AZURE_TOKEN=$("/mnt/c/Program Files (x86)/Microsoft SDKs/Azure/CLI2/wbin/az" account get-access-token --resource-type ms-graph | jq -r .accessToken)
export AAS_CAPSVC_TOKEN=$(ded.exe token -a capsvc -p)
