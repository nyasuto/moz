#!/bin/bash

if [ $# -ne 2 ]; then
    echo "Usage: $0 <key> <value>"
    exit 1
fi

key="$1"
value="$2"

if [[ "$key" =~ $'\t' ]]; then
    echo "Error: Key cannot contain tab characters"
    exit 1
fi

echo -e "${key}\t${value}" >>moz.log
