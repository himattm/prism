#!/bin/bash
#
# Test suite for install.sh hook merging logic
#
# Usage: ./test-hooks-merge.sh
#

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
DIM='\033[2m'
RESET='\033[0m'

PASSED=0
FAILED=0

# The merge logic extracted from install.sh
run_merge() {
    local input_file="$1"
    local output_file="$2"

    # Define prism hooks to add
    # Format: "EventName:prism-hook-command:async" (async is optional)
    local -a PRISM_HOOKS=(
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
    local MERGED=$(jq '.statusLine = {"type": "command", "command": "$HOME/.claude/prism"}' "$input_file")

    # Ensure hooks object exists
    MERGED=$(echo "$MERGED" | jq '.hooks //= {}')

    # Add each prism hook if not already present
    for hook_def in "${PRISM_HOOKS[@]}"; do
        # Parse: EVENT:HOOK_CMD:ASYNC_FLAG
        local EVENT="${hook_def%%:*}"
        local REST="${hook_def#*:}"
        local HOOK_CMD="${REST%%:*}"
        local ASYNC_FLAG="${REST##*:}"
        local PRISM_CMD="\$HOME/.claude/prism hook $HOOK_CMD"

        # Check if this prism command already exists in the hook
        local HAS_PRISM=$(echo "$MERGED" | jq --arg event "$EVENT" --arg cmd "$PRISM_CMD" '
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

    echo "$MERGED" | jq --sort-keys '.' > "$output_file"
}

# Test helper: check if all prism hooks are present
has_all_prism_hooks() {
    local file="$1"
    local hooks=(
        "UserPromptSubmit:busy"
        "Stop:idle"
        "SessionStart:session-start"
        "SessionEnd:session-end"
        "PreCompact:pre-compact"
        "Setup:setup"
        "PreToolUse:pre-tool-use"
        "PostToolUse:post-tool-use"
        "PermissionRequest:permission-request"
        "Notification:notification"
        "SubagentStop:subagent-stop"
    )

    for hook_def in "${hooks[@]}"; do
        local event="${hook_def%%:*}"
        local cmd="${hook_def##*:}"
        local prism_cmd="\$HOME/.claude/prism hook $cmd"

        local found=$(jq --arg event "$event" --arg cmd "$prism_cmd" '
            .hooks[$event] // [] |
            any(.[]; .hooks[]? | select(.command == $cmd))
        ' "$file")

        if [ "$found" != "true" ]; then
            echo "Missing: $event -> $prism_cmd"
            return 1
        fi
    done
    return 0
}

# Test helper: check statusLine is set correctly
has_prism_statusline() {
    local file="$1"
    local cmd=$(jq -r '.statusLine.command // ""' "$file")
    [ "$cmd" = '$HOME/.claude/prism' ]
}

# Test helper: count hooks for an event
count_hooks_for_event() {
    local file="$1"
    local event="$2"
    jq --arg event "$event" '.hooks[$event] | length' "$file"
}

# Test helper: check if a specific command exists in hooks
has_hook_command() {
    local file="$1"
    local event="$2"
    local cmd="$3"
    local found=$(jq --arg event "$event" --arg cmd "$cmd" '
        .hooks[$event] // [] |
        any(.[]; .hooks[]? | select(.command == $cmd))
    ' "$file")
    [ "$found" = "true" ]
}

# Test helper: check key exists
has_key() {
    local file="$1"
    local key="$2"
    jq --arg key "$key" 'has($key)' "$file" | grep -q true
}

# Test helper: get value
get_value() {
    local file="$1"
    local path="$2"
    jq -r "$path" "$file"
}

# Run a test
run_test() {
    local name="$1"
    local fixture="$2"
    shift 2
    local assertions=("$@")

    local output="$TMP_DIR/$(basename "$fixture" .json)-output.json"

    echo -n "  $name... "

    # Run merge
    if ! run_merge "$fixture" "$output" 2>/dev/null; then
        echo -e "${RED}FAILED${RESET} (merge error)"
        ((FAILED++))
        return
    fi

    # Run assertions
    local failed_assertions=()
    for assertion in "${assertions[@]}"; do
        if ! eval "$assertion" 2>/dev/null; then
            failed_assertions+=("$assertion")
        fi
    done

    if [ ${#failed_assertions[@]} -eq 0 ]; then
        echo -e "${GREEN}PASSED${RESET}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}FAILED${RESET}"
        for fa in "${failed_assertions[@]}"; do
            echo -e "    ${DIM}Failed: $fa${RESET}"
        done
        echo -e "    ${DIM}Output: $(cat "$output" | jq -c '.')${RESET}"
        FAILED=$((FAILED + 1))
    fi
}

echo ""
echo "Testing install.sh hook merge logic"
echo "===================================="
echo ""

# Test 1: Empty settings
run_test "01: Empty settings - adds all hooks" \
    "$FIXTURES_DIR/01-empty.json" \
    "has_all_prism_hooks '$TMP_DIR/01-empty-output.json'" \
    "has_prism_statusline '$TMP_DIR/01-empty-output.json'"

# Test 2: No hooks key
run_test "02: No hooks key - adds hooks object" \
    "$FIXTURES_DIR/02-no-hooks.json" \
    "has_all_prism_hooks '$TMP_DIR/02-no-hooks-output.json'" \
    "has_prism_statusline '$TMP_DIR/02-no-hooks-output.json'" \
    "[ \"\$(get_value '$TMP_DIR/02-no-hooks-output.json' '.theme')\" = 'dark' ]"

# Test 3: Empty hooks object
run_test "03: Empty hooks object - adds all prism hooks" \
    "$FIXTURES_DIR/03-empty-hooks.json" \
    "has_all_prism_hooks '$TMP_DIR/03-empty-hooks-output.json'" \
    "[ \"\$(get_value '$TMP_DIR/03-empty-hooks-output.json' '.theme')\" = 'dark' ]"

# Test 4: Partial prism hooks - adds missing ones
run_test "04: Partial prism hooks - adds missing" \
    "$FIXTURES_DIR/04-partial-prism-hooks.json" \
    "has_all_prism_hooks '$TMP_DIR/04-partial-prism-hooks-output.json'" \
    "[ \$(count_hooks_for_event '$TMP_DIR/04-partial-prism-hooks-output.json' 'Stop') -eq 1 ]" \
    "[ \$(count_hooks_for_event '$TMP_DIR/04-partial-prism-hooks-output.json' 'SessionStart') -eq 1 ]"

# Test 5: All prism hooks already present - idempotent
run_test "05: All hooks present - no duplicates (idempotent)" \
    "$FIXTURES_DIR/05-all-prism-hooks.json" \
    "has_all_prism_hooks '$TMP_DIR/05-all-prism-hooks-output.json'" \
    "[ \$(count_hooks_for_event '$TMP_DIR/05-all-prism-hooks-output.json' 'Stop') -eq 1 ]" \
    "[ \$(count_hooks_for_event '$TMP_DIR/05-all-prism-hooks-output.json' 'UserPromptSubmit') -eq 1 ]" \
    "[ \$(count_hooks_for_event '$TMP_DIR/05-all-prism-hooks-output.json' 'SessionStart') -eq 1 ]"

# Test 6: Custom hooks only - preserves them, adds prism
run_test "06: Custom hooks - preserved, prism added" \
    "$FIXTURES_DIR/06-custom-hooks-only.json" \
    "has_all_prism_hooks '$TMP_DIR/06-custom-hooks-only-output.json'" \
    "has_hook_command '$TMP_DIR/06-custom-hooks-only-output.json' 'Stop' \"notify-send 'Claude stopped'\"" \
    "has_hook_command '$TMP_DIR/06-custom-hooks-only-output.json' 'UserPromptSubmit' '/usr/local/bin/log-prompt.sh'" \
    "[ \$(count_hooks_for_event '$TMP_DIR/06-custom-hooks-only-output.json' 'Stop') -eq 2 ]"

# Test 7: Custom and prism hooks mixed - preserves all, no duplicates
run_test "07: Mixed hooks - no prism duplicates" \
    "$FIXTURES_DIR/07-custom-and-prism-hooks.json" \
    "has_all_prism_hooks '$TMP_DIR/07-custom-and-prism-hooks-output.json'" \
    "has_hook_command '$TMP_DIR/07-custom-and-prism-hooks-output.json' 'Stop' \"notify-send 'Claude stopped'\"" \
    "[ \$(count_hooks_for_event '$TMP_DIR/07-custom-and-prism-hooks-output.json' 'Stop') -eq 2 ]" \
    "[ \$(count_hooks_for_event '$TMP_DIR/07-custom-and-prism-hooks-output.json' 'UserPromptSubmit') -eq 1 ]"

# Test 8: Other statusLine - gets replaced
run_test "08: Other statusLine - replaced with prism" \
    "$FIXTURES_DIR/08-other-statusline.json" \
    "has_prism_statusline '$TMP_DIR/08-other-statusline-output.json'" \
    "has_all_prism_hooks '$TMP_DIR/08-other-statusline-output.json'" \
    "has_hook_command '$TMP_DIR/08-other-statusline-output.json' 'Stop' \"echo 'stopped'\""

# Test 9: Complex existing config - preserves everything
run_test "09: Complex config - preserves all settings" \
    "$FIXTURES_DIR/09-complex-existing.json" \
    "has_all_prism_hooks '$TMP_DIR/09-complex-existing-output.json'" \
    "[ \"\$(get_value '$TMP_DIR/09-complex-existing-output.json' '.theme')\" = 'dark' ]" \
    "[ \"\$(get_value '$TMP_DIR/09-complex-existing-output.json' '.apiKey')\" = 'sk-xxx' ]" \
    "has_key '$TMP_DIR/09-complex-existing-output.json' 'permissions'" \
    "has_hook_command '$TMP_DIR/09-complex-existing-output.json' 'Stop' 'python-linter.sh'" \
    "has_hook_command '$TMP_DIR/09-complex-existing-output.json' 'Stop' \"notify-send 'done'\"" \
    "has_hook_command '$TMP_DIR/09-complex-existing-output.json' 'PreToolUse' 'audit-tool-use.sh'"

# Test 10: Multiple hooks per event - all preserved
run_test "10: Multiple hooks per event - all preserved" \
    "$FIXTURES_DIR/10-multiple-hooks-per-event.json" \
    "has_all_prism_hooks '$TMP_DIR/10-multiple-hooks-per-event-output.json'" \
    "has_hook_command '$TMP_DIR/10-multiple-hooks-per-event-output.json' 'Stop' 'first-hook.sh'" \
    "has_hook_command '$TMP_DIR/10-multiple-hooks-per-event-output.json' 'Stop' 'second-hook.sh'" \
    "has_hook_command '$TMP_DIR/10-multiple-hooks-per-event-output.json' 'Stop' 'third-hook.sh'" \
    "[ \$(count_hooks_for_event '$TMP_DIR/10-multiple-hooks-per-event-output.json' 'Stop') -eq 3 ]"

# Test 11: Idempotency - run twice, same result
echo -n "  11: Idempotency - running twice produces same result... "
run_merge "$FIXTURES_DIR/06-custom-hooks-only.json" "$TMP_DIR/idem-1.json" 2>/dev/null
run_merge "$TMP_DIR/idem-1.json" "$TMP_DIR/idem-2.json" 2>/dev/null
if diff -q "$TMP_DIR/idem-1.json" "$TMP_DIR/idem-2.json" >/dev/null 2>&1; then
    echo -e "${GREEN}PASSED${RESET}"
    PASSED=$((PASSED + 1))
else
    echo -e "${RED}FAILED${RESET}"
    echo -e "    ${DIM}First run differs from second run${RESET}"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "===================================="
echo -e "Results: ${GREEN}$PASSED passed${RESET}, ${RED}$FAILED failed${RESET}"
echo ""

if [ $FAILED -gt 0 ]; then
    exit 1
fi
