#!/bin/bash

if [ $# -ne 1 ]; then
    echo "Usage: $0 <key>"
    exit 1
fi

key="$1"

if [[ "$key" =~ $'\t' ]]; then
    echo "Error: Key cannot contain tab characters"
    exit 1
fi

echo -e "${key}\t__DELETED__" >>moz.log
