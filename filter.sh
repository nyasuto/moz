#!/usr/bin/env bash

if [ $# -ne 1 ]; then
    echo "Usage: $0 <pattern>"
    exit 1
fi

pattern="$1"

if [ ! -f "moz.log" ]; then
    exit 0
fi

temp_file=$(mktemp)

while IFS=$'\t' read -r key value; do
    echo -e "${key}\t${value}" >> "$temp_file"
done < moz.log

awk -F'\t' -v pattern="$pattern" '{
    latest[$1] = $2
}
END {
    for (key in latest) {
        if (latest[key] != "__DELETED__" && index(key, pattern) > 0) {
            print key "\t" latest[key]
        }
    }
}' "$temp_file" | sort

rm "$temp_file"