#!/bin/bash
#
# Prism Installer
# A fast, customizable status line for Claude Code
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/himattm/prism/main/install.sh | bash
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
DIM='\033[2m'
RESET='\033[0m'

REPO="himattm/prism"
BRANCH="main"
CLAUDE_DIR="$HOME/.claude"
SETTINGS_FILE="$CLAUDE_DIR/settings.json"
GLOBAL_CONFIG="$CLAUDE_DIR/prism-config.json"

info() { echo -e "${CYAN}$1${RESET}"; }
success() { echo -e "${GREEN}$1${RESET}"; }
warn() { echo -e "${YELLOW}$1${RESET}"; }
error() { echo -e "${RED}$1${RESET}"; exit 1; }

# Version comparison: returns 0 if $1 < $2
version_lt() {
    [ "$1" = "$2" ] && return 1
    local IFS=.
    local i v1=($1) v2=($2)
    for ((i=0; i<${#v1[@]} || i<${#v2[@]}; i++)); do
        local n1=${v1[i]:-0}
        local n2=${v2[i]:-0}
        ((n1 < n2)) && return 0
        ((n1 > n2)) && return 1
    done
    return 1
}

# Get installed prism version (empty if not installed)
get_installed_version() {
    if [ -x "$CLAUDE_DIR/prism" ]; then
        "$CLAUDE_DIR/prism" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo ""
    else
        echo ""
    fi
}

# Remove a section from config file
# Usage: remove_section "mcp" "/path/to/config.json"
remove_section() {
    local section="$1"
    local config_file="$2"

    [ ! -f "$config_file" ] && return

    # Check if section exists in the file
    if grep -q "\"$section\"" "$config_file" 2>/dev/null; then
        # Use jq to remove the section from arrays (handles both flat and nested)
        local tmp=$(mktemp)
        jq --arg s "$section" '
            if .sections then
                .sections |= (
                    if type == "array" then
                        if (.[0] | type) == "array" then
                            # Nested array: [[...], [...]]
                            map(map(select(. != $s)))
                        else
                            # Flat array: [...]
                            map(select(. != $s))
                        end
                    else
                        .
                    end
                )
            else
                .
            end
        ' "$config_file" > "$tmp" && mv "$tmp" "$config_file"
        return 0
    fi
    return 1
}

# Rename a section in config file
# Usage: rename_section "old" "new" "/path/to/config.json"
rename_section() {
    local old_name="$1"
    local new_name="$2"
    local config_file="$3"

    [ ! -f "$config_file" ] && return

    if grep -q "\"$old_name\"" "$config_file" 2>/dev/null; then
        local tmp=$(mktemp)
        jq --arg old "$old_name" --arg new "$new_name" '
            if .sections then
                .sections |= (
                    if type == "array" then
                        if (.[0] | type) == "array" then
                            map(map(if . == $old then $new else . end))
                        else
                            map(if . == $old then $new else . end)
                        end
                    else
                        .
                    end
                )
            else
                .
            end
        ' "$config_file" > "$tmp" && mv "$tmp" "$config_file"
        return 0
    fi
    return 1
}

# Add a section to config file (on a new line for multi-line, or appended for flat)
# Usage: add_section "spotify" "/path/to/config.json"
add_section() {
    local section="$1"
    local config_file="$2"

    [ ! -f "$config_file" ] && return 1

    # Check if section already exists
    if grep -q "\"$section\"" "$config_file" 2>/dev/null; then
        return 1  # Already exists
    fi

    local tmp=$(mktemp)
    jq --arg s "$section" '
        if .sections then
            .sections |= (
                if type == "array" then
                    if (.[0] | type) == "array" then
                        # Nested array: add as new line
                        . + [[$s]]
                    else
                        # Flat array: convert to nested with new line
                        [., [$s]]
                    end
                else
                    .
                end
            )
        else
            # No sections, create with the new section on line 2
            .sections = [["dir", "model", "context", "usage", "git"], [$s]]
        end
    ' "$config_file" > "$tmp" && mv "$tmp" "$config_file"
    return 0
}

# Get config version from config file (empty if not set)
get_config_version() {
    local config_file="$1"
    [ ! -f "$config_file" ] && echo "" && return
    jq -r '.configVersion // ""' "$config_file" 2>/dev/null || echo ""
}

# Set config version in config file
set_config_version() {
    local config_file="$1"
    local new_version="$2"
    [ ! -f "$config_file" ] && return
    local tmp=$(mktemp)
    jq --arg v "$new_version" '.configVersion = $v' "$config_file" > "$tmp" && mv "$tmp" "$config_file"
}

# Run migrations based on CONFIG version (not binary version)
# This ensures we don't re-apply migrations to user-customized configs
run_migrations() {
    local migrated=false

    # No config file - no migrations needed (fresh install will create it)
    [ ! -f "$GLOBAL_CONFIG" ] && return

    # Get config version (what migrations have been applied)
    local config_version
    config_version=$(get_config_version "$GLOBAL_CONFIG")

    # If no config version, this is a pre-0.7.0 config - use binary version as baseline
    if [ -z "$config_version" ]; then
        config_version="$1"  # Fall back to old binary version for legacy configs
        [ -z "$config_version" ] && config_version="0.0.0"
    fi

    info "Checking for config migrations (config version: ${config_version:-none})..."

    # ============================================================
    # MIGRATIONS - Add new migrations at the bottom
    # Migrations run based on configVersion, not binary version.
    # Once a migration runs, configVersion is updated so it won't run again.
    # ============================================================

    # v0.4.0: Remove gradle and xcode plugins
    if version_lt "$config_version" "0.4.0"; then
        if remove_section "gradle" "$GLOBAL_CONFIG"; then
            success "  Migrated: removed 'gradle' section (plugin removed)"
            migrated=true
        fi
        if remove_section "xcode" "$GLOBAL_CONFIG"; then
            success "  Migrated: removed 'xcode' section (plugin removed)"
            migrated=true
        fi
        if remove_section "mcp" "$GLOBAL_CONFIG"; then
            success "  Migrated: removed 'mcp' section (plugin removed)"
            migrated=true
        fi
    fi

    # v0.7.0: Add spotify plugin (on second line)
    if version_lt "$config_version" "0.7.0"; then
        if add_section "spotify" "$GLOBAL_CONFIG"; then
            success "  Migrated: added 'spotify' section (new plugin)"
            migrated=true
        fi
    fi

    # v0.11.0: Add supabase and vercel plugins
    # System metrics (cpu, memory, battery) and stack are opt-in
    if version_lt "$config_version" "0.11.0"; then
        for p in supabase vercel; do
            if add_section "$p" "$GLOBAL_CONFIG"; then
                success "  Migrated: added '$p' section (new plugin)"
                migrated=true
            fi
        done
    fi

    # v0.11.1: Add peak hours and android plugins
    if version_lt "$config_version" "0.11.1"; then
        for p in peakhours android; do
            if add_section "$p" "$GLOBAL_CONFIG"; then
                success "  Migrated: added '$p' section (new plugin)"
                migrated=true
            fi
        done
    fi

    # ============================================================

    # Update config version to current so migrations don't re-run
    local current_version
    current_version=$("$CLAUDE_DIR/prism" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "0.7.0")
    set_config_version "$GLOBAL_CONFIG" "$current_version"

    if [ "$migrated" = false ]; then
        echo -e "  ${DIM}No migrations needed${RESET}"
    else
        echo -e "  ${DIM}Updated configVersion to $current_version${RESET}"
    fi
}

echo ""
echo -e "${CYAN}💎 Prism Installer${RESET}"
echo -e "${DIM}A fast, customizable status line for Claude Code${RESET}"
echo ""

# Check for dependencies
if ! command -v jq &> /dev/null; then
    error "jq is required but not installed. Install it with: brew install jq"
fi

if ! command -v curl &> /dev/null; then
    error "curl is required but not installed."
fi

# Create ~/.claude if it doesn't exist
if [ ! -d "$CLAUDE_DIR" ]; then
    info "Creating $CLAUDE_DIR..."
    mkdir -p "$CLAUDE_DIR"
fi

# Capture current version before upgrade (for migrations)
OLD_VERSION=$(get_installed_version)
if [ -n "$OLD_VERSION" ]; then
    echo -e "  ${DIM}Current version: $OLD_VERSION${RESET}"
fi

# Download Go binary
info "Downloading Prism binary..."

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
esac

BINARY_URL="https://github.com/$REPO/releases/latest/download/prism-${OS}-${ARCH}"

# Use atomic update: download to temp file, then mv (prevents corruption if prism is running)
if curl -fsSL "$BINARY_URL" -o "$CLAUDE_DIR/prism.new" 2>/dev/null; then
    chmod +x "$CLAUDE_DIR/prism.new"
    mv "$CLAUDE_DIR/prism.new" "$CLAUDE_DIR/prism"
    success "  Downloaded prism binary (${OS}-${ARCH})"
else
    rm -f "$CLAUDE_DIR/prism.new" 2>/dev/null

    # Try to build from source if Go is installed
    if command -v go &> /dev/null; then
        info "  Pre-built binary not available, building from source..."
        TMP_DIR=$(mktemp -d)
        trap "rm -rf $TMP_DIR" EXIT

        curl -fsSL "https://github.com/$REPO/archive/$BRANCH.tar.gz" | tar -xz -C "$TMP_DIR"
        cd "$TMP_DIR/prism-$BRANCH"
        go build -o "$CLAUDE_DIR/prism.new" ./cmd/prism/
        cd - > /dev/null

        chmod +x "$CLAUDE_DIR/prism.new"
        mv "$CLAUDE_DIR/prism.new" "$CLAUDE_DIR/prism"
        success "  Built prism binary from source"
    else
        error "Pre-built binary not available for ${OS}-${ARCH} and Go is not installed to build from source."
    fi
fi

# Clean up old hook scripts if they exist
if [ -f "$CLAUDE_DIR/prism-idle-hook.sh" ] || [ -f "$CLAUDE_DIR/prism-busy-hook.sh" ] || [ -f "$CLAUDE_DIR/prism-update-hook.sh" ]; then
    info "Removing old hook scripts (now built into prism binary)..."
    rm -f "$CLAUDE_DIR/prism-idle-hook.sh" "$CLAUDE_DIR/prism-busy-hook.sh" "$CLAUDE_DIR/prism-update-hook.sh"
    success "  Cleaned up legacy hook scripts"
fi

# Run config migrations if upgrading
run_migrations "$OLD_VERSION"

# Create global config if it doesn't exist
if [ ! -f "$GLOBAL_CONFIG" ]; then
    info "Creating global config..."
    "$CLAUDE_DIR/prism" init-global 2>/dev/null || true
    success "  Created $GLOBAL_CONFIG"
fi

# Update settings.json
info "Configuring Claude Code settings..."

if [ ! -f "$SETTINGS_FILE" ]; then
    # Create new settings file with all hooks
    cat > "$SETTINGS_FILE" << 'EOF'
{
  "statusLine": {
    "type": "command",
    "command": "$HOME/.claude/prism"
  },
  "hooks": {
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook busy"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook idle", "async": true}]}],
    "SessionStart": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook session-start", "async": true}]}],
    "SessionEnd": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook session-end", "async": true}]}],
    "PreCompact": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook pre-compact"}]}],
    "Setup": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook setup"}]}],
    "PreToolUse": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook pre-tool-use"}]}],
    "PostToolUse": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook post-tool-use", "async": true}]}],
    "PermissionRequest": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook permission-request"}]}],
    "Notification": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook notification", "async": true}]}],
    "SubagentStop": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook subagent-stop", "async": true}]}]
  }
}
EOF
    success "  Created $SETTINGS_FILE"
else
    # Merge with existing settings (preserving custom hooks)
    BACKUP_FILE="$SETTINGS_FILE.backup.$(date +%s)"
    cp "$SETTINGS_FILE" "$BACKUP_FILE"

    # Define all prism hooks to add
    # Format: "EventName:prism-hook-command:async" (async is optional)
    PRISM_HOOKS=(
        "UserPromptSubmit:busy:"
        "Stop:idle:async"
        "SessionStart:session-start:async"
        "SessionEnd:session-end:async"
        "PreCompact:pre-compact:"
        "Setup:setup:"
        "PreToolUse:pre-tool-use:"
        "PostToolUse:post-tool-use:async"
        "PermissionRequest:permission-request:"
        "Notification:notification:async"
        "SubagentStop:subagent-stop:async"
    )

    # Start with existing settings, add statusLine
    MERGED=$(jq '.statusLine = {"type": "command", "command": "$HOME/.claude/prism"}' "$SETTINGS_FILE")

    # Ensure hooks object exists
    MERGED=$(echo "$MERGED" | jq '.hooks //= {}')

    # Add each prism hook if not already present
    for hook_def in "${PRISM_HOOKS[@]}"; do
        # Parse: EVENT:HOOK_CMD:ASYNC_FLAG
        EVENT="${hook_def%%:*}"
        REST="${hook_def#*:}"
        HOOK_CMD="${REST%%:*}"
        ASYNC_FLAG="${REST##*:}"
        PRISM_CMD="\$HOME/.claude/prism hook $HOOK_CMD"

        # Check if this prism command already exists in the hook
        HAS_PRISM=$(echo "$MERGED" | jq --arg event "$EVENT" --arg cmd "$PRISM_CMD" '
            .hooks[$event] // [] |
            any(.[]; .hooks[]? | select(.command == $cmd))
        ')

        if [ "$HAS_PRISM" = "false" ]; then
            # Add prism hook to this event (append to existing array or create new)
            if [ "$ASYNC_FLAG" = "async" ]; then
                MERGED=$(echo "$MERGED" | jq --arg event "$EVENT" --arg cmd "$PRISM_CMD" '
                    .hooks[$event] = (.hooks[$event] // []) + [{"hooks": [{"type": "command", "command": $cmd, "async": true}]}]
                ')
            else
                MERGED=$(echo "$MERGED" | jq --arg event "$EVENT" --arg cmd "$PRISM_CMD" '
                    .hooks[$event] = (.hooks[$event] // []) + [{"hooks": [{"type": "command", "command": $cmd}]}]
                ')
            fi
        fi
    done

    echo "$MERGED" > "$SETTINGS_FILE"

    success "  Updated $SETTINGS_FILE (preserved existing hooks)"
    echo -e "  ${DIM}Backup saved to $BACKUP_FILE${RESET}"
fi

echo ""
success "Prism installed successfully!"
echo ""
echo "Restart Claude Code or start a new session to activate."
echo ""
echo -e "${CYAN}Configuration${RESET} (highest to lowest priority):"
echo -e "  ${DIM}1.${RESET} .claude/prism.local.json    ${DIM}Your personal overrides (gitignored)${RESET}"
echo -e "  ${DIM}2.${RESET} .claude/prism.json          ${DIM}Repo config (commit for your team)${RESET}"
echo -e "  ${DIM}3.${RESET} ~/.claude/prism-config.json ${DIM}Your global defaults${RESET}"
echo ""
echo -e "${CYAN}Optional:${RESET} Create a repo-specific config with:"
echo "  ~/.claude/prism init"
echo ""
