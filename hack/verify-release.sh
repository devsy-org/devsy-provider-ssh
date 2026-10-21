#!/usr/bin/env bash
set -euo pipefail

tag="${1:?expected release tag}"
dist="${2:?expected distribution directory}"
expected=(
    devsy-provider-ssh-darwin-amd64
    devsy-provider-ssh-darwin-arm64
    devsy-provider-ssh-linux-amd64
    devsy-provider-ssh-linux-arm64
    devsy-provider-ssh-windows-amd64.exe
)
release="$(gh release view "$tag" --json assets)"
for name in "${expected[@]}" checksums.txt provider.yaml; do
    if ! jq -e --arg name "$name" '[.assets[] | select(.name == $name and .size > 0)] | length == 1' <<<"$release" >/dev/null; then
        echo "Missing or empty release asset: $name" >&2
        exit 1
    fi
done

download="$(mktemp -d)"
trap 'rm -rf "$download"' EXIT
for name in "${expected[@]}" checksums.txt provider.yaml; do
    gh release download "$tag" --pattern "$name" --dir "$download"
done
cmp "$dist/checksums.txt" "$download/checksums.txt"
cmp "$dist/provider.yaml" "$download/provider.yaml"
printf '%s\n' "${expected[@]}" | sort >"$download/expected.txt"
awk '{print $2}' "$download/checksums.txt" | sort >"$download/actual.txt"
diff -u "$download/expected.txt" "$download/actual.txt"
(cd "$download" && sha256sum --strict --check checksums.txt)
echo "Verified all seven release assets for $tag"
