#!/bin/sh

set -eu

wire_file="api_provider_wire_gen.go"
generated_directive="//go:generate go run -mod=mod github.com/google/wire/cmd/wire"
reproducible_directive="//go:generate sh ../../../generate_api_provider_wire.sh"
temporary_file="${wire_file}.tmp"

go run -mod=mod github.com/google/wire/cmd/wire gen -output_file_prefix api_provider_ .
awk -v generated="$generated_directive" -v reproducible="$reproducible_directive" '
	$0 == generated { print reproducible; next }
	{ print }
' "$wire_file" >"$temporary_file"
mv "$temporary_file" "$wire_file"
