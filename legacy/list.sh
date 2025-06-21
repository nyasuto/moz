#!/usr/bin/env bash

if [ ! -f "moz.log" ]; then
    exit 0
fi

temp_file=$(mktemp)

while IFS=$'\t' read -r key value; do
    echo -e "${key}\t${value}" >>"$temp_file"
done <moz.log

awk -F'\t' '{
    latest[$1] = $2
}
END {
    for (key in latest) {
        if (latest[key] != "__DELETED__") {
            print key "\t" latest[key]
        }
    }
}' "$temp_file" | sort

rm "$temp_file"
