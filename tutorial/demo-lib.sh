# ConfigHub demo runner.
#
# Types a command out, waits for you to press a key, runs it. Nothing else --
# no pv, no tmux, no script(1), and nothing newer than bash 3.2, which is what
# macOS ships.
#
# Use it like this:
#
#   #!/usr/bin/env bash
#   source "$(dirname "${BASH_SOURCE[0]}")/demo-lib.sh"
#
#   desc "Everything starts from the base."
#   run  "cub unit list --space cubbychat-base"
#
# Keys, while a command is on screen waiting:
#
#   space, enter   run it
#   s              skip it -- leave it unrun and move on
#   f              stop typing character by character for the rest of the demo
#   !              drop into a subshell; exit that shell to come back here
#   q              quit
#
# Environment:
#
#   DEMO_SPEED=N   characters per second while typing (default 45; 0 = instant)
#   DEMO_AUTO=1    never wait for a key -- for rehearsing a script end to end
#   DEMO_DRYRUN=1  show every command but run none of them
#   NO_COLOR=1     plain text
#
#   DEMO_SETUP=INIT  do the demo's slow setup up front, before the narration
#                    starts, and describe it when the demo reaches it instead
#                    of running it there. Which commands are setup is the
#                    demo script's business -- a cluster to build, a database
#                    to seed, an image to pull; the library fixes the name so
#                    every demo answers to the same one. Run those commands
#                    with run_now, and branch on the variable wherever the
#                    narration would otherwise run them.
#
# Commands run in *this* shell, not a subshell, so `source some.env` and `cd`
# affect the commands after them -- which a demo of a real workflow needs.

# Deliberately no `set -e`: a command that fails mid-demo should show its exit
# code and let you carry on talking, not kill the presentation.

DEMO_SPEED="${DEMO_SPEED-45}"
DEMO_AUTO="${DEMO_AUTO-}"
DEMO_DRYRUN="${DEMO_DRYRUN-}"
DEMO_SETUP="${DEMO_SETUP-}"

_demo_fast=""       # set by 'f', or by DEMO_SPEED=0

if [ -t 1 ] && [ -z "${NO_COLOR-}" ] && command -v tput >/dev/null 2>&1; then
    _c_cmd="$(tput bold; tput setaf 2)"     # green: what you type
    _c_desc="$(tput setaf 6)"               # cyan: narration
    _c_prompt="$(tput bold; tput setaf 3)"  # yellow: the prompt
    _c_warn="$(tput bold; tput setaf 1)"    # red: a non-zero exit
    _c_dim="$(tput setaf 8)"
    _c_off="$(tput sgr0)"
else
    _c_cmd="" _c_desc="" _c_prompt="" _c_warn="" _c_dim="" _c_off=""
fi

# _demo_nap sleeps for a fractional number of seconds. Both BSD and GNU sleep
# take a decimal; this exists so the delay is in one place.
_demo_nap() {
    sleep "$1" 2>/dev/null || true
}

# _demo_type writes text one character at a time, so a command appears the way
# it would if someone were typing it. Falls back to writing it whole when the
# demo is running fast, non-interactively, or at DEMO_SPEED=0.
_demo_type() {
    _text="$1"
    if [ -n "$_demo_fast" ] || [ -n "$DEMO_AUTO" ] || [ ! -t 1 ] || \
       [ "$DEMO_SPEED" = "0" ]; then
        printf '%s' "$_text"
        return
    fi
    _delay=$(awk -v s="$DEMO_SPEED" 'BEGIN { if (s <= 0) print 0; else printf "%.4f", 1/s }')
    _i=0
    _len=${#_text}
    while [ "$_i" -lt "$_len" ]; do
        printf '%s' "${_text:$_i:1}"
        _demo_nap "$_delay"
        _i=$((_i + 1))
    done
}

_demo_prompt() {
    printf '%s$%s ' "$_c_prompt" "$_c_off"
}

# _demo_key waits for a single keypress and sets _demo_action to what should
# happen to the command on screen: "run", "skip", or "quit". It reports through
# a variable rather than stdout because stdout is the demo.
#
# The keys that are about the demo itself -- f, ! -- are handled here and do not
# end the wait.
_demo_key() {
    _demo_action=run
    if [ -n "$DEMO_AUTO" ] || [ ! -t 0 ]; then
        printf '\n'
        return
    fi
    while true; do
        _key=""
        IFS= read -r -s -n 1 _key
        case "$_key" in
            q|Q)
                printf '\n'
                _demo_action=quit
                return
                ;;
            s|S)
                printf '   %s(skipped)%s\n' "$_c_dim" "$_c_off"
                _demo_action=skip
                return
                ;;
            f|F)
                _demo_fast=1
                ;;
            '!')
                printf '\n%s-- subshell; exit to return to the demo --%s\n' "$_c_dim" "$_c_off"
                "${SHELL:-/bin/bash}"
                printf '%s-- back --%s\n' "$_c_dim" "$_c_off"
                _demo_prompt
                _demo_type "$_demo_current"
                ;;
            *)
                # space, enter, anything else: go.
                printf '\n'
                return
                ;;
        esac
    done
}

# desc prints narration. It is what you would be saying out loud, so it reads
# as a comment above the command rather than as output.
desc() {
    printf '%s# %s%s\n' "$_c_desc" "$*" "$_c_off"
}

# heading marks a section of the demo.
heading() {
    printf '\n%s%s%s\n' "$_c_prompt" "=== $* ===" "$_c_off"
}

# pause waits for a keypress without running anything -- a beat while you talk
# about what just came back.
pause() {
    [ -n "$DEMO_AUTO" ] || [ ! -t 0 ] && return 0
    printf '%s%s%s' "$_c_dim" "${1:-...}" "$_c_off"
    IFS= read -r -s -n 1 _ || true
    printf '\r%*s\r' "${#1}" ""
}

# run types a command, waits for a key, and runs it here. The exit code is
# reported when it is not zero rather than being swallowed or fatal.
run() {
    _demo_current="$*"
    _demo_prompt
    printf '%s' "$_c_cmd"
    _demo_type "$_demo_current"
    printf '%s' "$_c_off"

    _demo_key
    case "$_demo_action" in
        quit)
            printf '%s-- demo ended --%s\n' "$_c_dim" "$_c_off"
            exit 0
            ;;
        skip)
            return 0
            ;;
    esac

    if [ -n "$DEMO_DRYRUN" ]; then
        printf '%s(dry run: not executed)%s\n' "$_c_dim" "$_c_off"
        return 0
    fi
    eval "$_demo_current"
    _rc=$?
    if [ "$_rc" -ne 0 ]; then
        printf '%s[exit %d]%s\n' "$_c_warn" "$_rc" "$_c_off"
    fi
    return 0
}

# run_now shows a command the way run does and runs it straight away, without
# waiting for a key -- for setup that happens before the demo starts, where
# there is nobody to press it. See DEMO_SETUP.
run_now() {
    _demo_prompt
    printf '%s%s%s\n' "$_c_cmd" "$*" "$_c_off"

    if [ -n "$DEMO_DRYRUN" ]; then
        printf '%s(dry run: not executed)%s\n' "$_c_dim" "$_c_off"
        return 0
    fi
    eval "$@"
    _rc=$?
    if [ "$_rc" -ne 0 ]; then
        printf '%s[exit %d]%s\n' "$_c_warn" "$_rc" "$_c_off"
    fi
    return 0
}

# silent runs a command without showing it -- setup a demo needs but nobody
# wants to watch.
silent() {
    [ -n "$DEMO_DRYRUN" ] && return 0
    eval "$@" >/dev/null 2>&1
    return 0
}

# demo_end exists so a script can mark its own end; the runner leaves the
# terminal on a fresh line already.
demo_end() {
    :
}
