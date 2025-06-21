#!/bin/bash

if [ $# -ne 1 ]; then
    echo "Usage: $0 <key>"
    exit 1
fi

key="$1"

if [ ! -f "moz.log" ]; then
    exit 1
fi

value=$(grep "^${key}	" moz.log | tail -1 | cut -f2)

if [ "$value" = "__DELETED__" ]; then
    exit 1
fi

if [ -n "$value" ]; then
    echo "$value"
else
    exit 1
fi
