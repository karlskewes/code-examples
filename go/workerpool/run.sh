#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

# Simple task runner that's not Make so syntax highlighting and shellcheck, shfmt, etc work.

http() { ## curl localhost to trigger scrypt.Key()
	curl localhost:8080 "$@"
}

trace() { ## Open trace.out
	go tool trace trace.out "$@"
}

start() { ## Start without forced runtime.GC()
	go run main.go "$@"
}

startf() { ## Start with forced runtime.GC()
	go run main.go -force "$@"
}

k() { ## Simulate load, start with -count <arg>
	if [ $# -ne 1 ]; then
		echo 1>&2 "Usage: $0 ${FUNCNAME[0]} <arg>"
                exit 3
        fi

	go run main.go -count "$1" "$@"
}

kf() { ## Simulate load, start with -count <arg> -force
	if [ $# -ne 1 ]; then
		echo 1>&2 "Usage: $0 ${FUNCNAME[0]} <arg>"
                exit 3
        fi

	go run main.go -count "$1" -force "$@"
}

z_debug() { ## Debug REPL
	echo "Stopped in REPL. Press ^D to resume, or ^C to abort."
	local line
	while read -r -p "> " line; do
		eval "$line"
	done
	echo
}

help() { ## Display usage for this application
	echo "$0 <task> <args>"
	grep -E '^[0-9a-zA-Z_-]+\(\) { ## .*$' "$0" |
		sed -E 's/\(\)//' |
		sort |
		awk 'BEGIN {FS = "{.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $1, $2}'
}

TIMEFORMAT="Task completed in %3lR"
time "${@:-help}"
