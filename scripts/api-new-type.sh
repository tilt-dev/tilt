#!/bin/bash
#
# Generates a new datatype for the Tilt API server.

set -e

DIR=$(dirname "$0")
cd "$DIR/.."

TYPE_NAME="$1"
if [[ "$TYPE_NAME" == "" ]]; then
    echo "Usage: api-new-type.sh [TypeName]"
    exit 1
fi

# Each type name has two forms
# - the kind (DoThing)
# - the resource (dothing)
# where the first is used in YAML and the second is used in HTTP endpoints
#
# shellcheck disable=SC2001
TYPE_NAME_LOWER=$(echo "$TYPE_NAME" | sed -e 's/^\(.*\)$/\L\1/g')
if [[ "$TYPE_NAME" == "$TYPE_NAME_LOWER" ]]; then
    echo "Error: type name must be uppercase"
    exit 1
fi

OUTPUT_FILE=pkg/apis/core/v1alpha1/"$TYPE_NAME_LOWER"_types.go
sed -e "s/Manifest/$TYPE_NAME/g" scripts/api-new-type-boilerplate.go.txt | \
    sed -e "s/manifest/$TYPE_NAME_LOWER/g" > \
    "$OUTPUT_FILE"

echo "Successfully generated $TYPE_NAME: $OUTPUT_FILE"
echo "Please add it to the list of types in pkg/apis/core/v1alpha1/register.go"
echo "To generate clients for your new type:"
echo "> make update-codegen"
