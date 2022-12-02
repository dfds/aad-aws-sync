#!/bin/sh

export AAS_AZURE_TOKEN=$(az account get-access-token --resource-type ms-graph | jq -r .accessToken)
export AAS_CAPSVC_TOKEN=$(ded token -a capsvc -p)
