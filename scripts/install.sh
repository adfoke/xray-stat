#!/usr/bin/env sh

set -eu

REPO="${REPO:-adfoke/xray-stat}"
BINARY_NAME="${BINARY_NAME:-xray-stat}"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-}"

fail() {
	echo "install.sh: $*" >&2
	exit 1
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

detect_os() {
	case "$(uname -s)" in
		Darwin) echo "darwin" ;;
		*) fail "unsupported operating system: $(uname -s)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		arm64|aarch64) echo "arm64" ;;
		x86_64|amd64) echo "amd64" ;;
		*) fail "unsupported architecture: $(uname -m)" ;;
	esac
}

default_install_dir() {
	if [ -d /usr/local/bin ] || [ ! -e /usr/local/bin ]; then
		echo "/usr/local/bin"
	elif [ -d /opt/homebrew/bin ]; then
		echo "/opt/homebrew/bin"
	else
		echo "$HOME/.local/bin"
	fi
}

resolve_version() {
	if [ "$VERSION" != "latest" ]; then
		echo "$VERSION"
		return
	fi

	tag=$(
		curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
			sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
			head -n 1
	)
	[ -n "$tag" ] || fail "failed to resolve latest release tag"
	echo "$tag"
}

prepare_install_dir() {
	target_dir=$1

	if [ -w "$target_dir" ] 2>/dev/null; then
		mkdir -p "$target_dir"
		echo ""
		return
	fi

	parent_dir=$(dirname "$target_dir")
	if [ ! -e "$target_dir" ] && [ -w "$parent_dir" ]; then
		mkdir -p "$target_dir"
		echo ""
		return
	fi

	if command -v sudo >/dev/null 2>&1; then
		sudo mkdir -p "$target_dir"
		echo "sudo"
		return
	fi

	if [ -z "$INSTALL_DIR" ]; then
		fallback_dir="$HOME/.local/bin"
		mkdir -p "$fallback_dir"
		INSTALL_DIR="$fallback_dir"
		echo ""
		return
	fi

	fail "install directory is not writable: $target_dir"
}

need_cmd curl
need_cmd tar
need_cmd shasum
need_cmd install

OS=$(detect_os)
ARCH=$(detect_arch)
RESOLVED_VERSION=$(resolve_version)
ASSET_NAME="${BINARY_NAME}_${RESOLVED_VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$RESOLVED_VERSION"
ASSET_URL="$BASE_URL/$ASSET_NAME"
CHECKSUM_URL="$BASE_URL/SHA256SUMS"

if [ -z "$INSTALL_DIR" ]; then
	INSTALL_DIR=$(default_install_dir)
fi

SUDO_CMD=$(prepare_install_dir "$INSTALL_DIR")

TMP_DIR=$(mktemp -d)
cleanup() {
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

echo "Downloading $ASSET_NAME"
curl -fsSL -o "$TMP_DIR/$ASSET_NAME" "$ASSET_URL"
curl -fsSL -o "$TMP_DIR/SHA256SUMS" "$CHECKSUM_URL"

EXPECTED_SUM=$(awk "/  $ASSET_NAME\$/ {print \$1}" "$TMP_DIR/SHA256SUMS")
[ -n "$EXPECTED_SUM" ] || fail "checksum entry not found for $ASSET_NAME"

ACTUAL_SUM=$(shasum -a 256 "$TMP_DIR/$ASSET_NAME" | awk '{print $1}')
[ "$EXPECTED_SUM" = "$ACTUAL_SUM" ] || fail "checksum verification failed"

tar -xzf "$TMP_DIR/$ASSET_NAME" -C "$TMP_DIR"
[ -f "$TMP_DIR/$BINARY_NAME" ] || fail "archive did not contain $BINARY_NAME"

if [ -n "$SUDO_CMD" ]; then
	sudo install -m 755 "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
	install -m 755 "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
fi

echo "Installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"

case ":$PATH:" in
	*:"$INSTALL_DIR":*)
		;;
	*)
		echo "Note: $INSTALL_DIR is not in PATH"
		echo "Add this to your shell config:"
		echo "export PATH=\"$INSTALL_DIR:\$PATH\""
		;;
esac
