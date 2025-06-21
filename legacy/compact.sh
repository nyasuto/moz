#!/usr/bin/env bash

if [ ! -f "moz.log" ]; then
    exit 0
fi

temp_file=$(mktemp)
backup_file=$(mktemp)

cp moz.log "$backup_file"

awk -F'\t' '{
    latest[$1] = $2
}
END {
    for (key in latest) {
        if (latest[key] != "__DELETED__") {
            print key "\t" latest[key]
        }
    }
}' "$backup_file" | sort >"$temp_file"

mv "$temp_file" moz.log
rm "$backup_file"
