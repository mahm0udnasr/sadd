# Sourced (hidden) at the start of every tape. It makes the recording safe:
# sadd works on a throwaway hosts file, so nothing on your system changes
# and no password is needed. Run tapes from the repo root after "make".
export HOSTS_FILE="$(mktemp -d)/hosts"
printf '127.0.0.1 localhost\n::1 localhost ip6-localhost\n' > "$HOSTS_FILE"
export PATH="$PWD:$PATH"   # use the freshly built ./sadd

# "sudo" is only shown so the commands look like real use; with a test
# file sadd does not need root, so just run the command.
sudo() { "$@"; }

# Hide the notices about the test file ("(using HOSTS_FILE=...)" and
# "... is not the system hosts file") so the output looks like real use.
# The TUI (no arguments, or "tui") needs a real terminal, so it is not piped.
sadd() {
  if [ $# -eq 0 ] || [ "$1" = tui ]; then command sadd "$@" 2>/dev/null
  else command sadd "$@" 2>&1 | grep -v -e '^(using HOSTS_FILE' -e 'is not the system hosts file'; fi
}

# Remove the systemd user timers that "--for" created during the recording.
cleanup() { systemctl --user stop 'sadd-expire-*.timer' 2>/dev/null; }
trap cleanup EXIT

export PS1='\[\e[1;35m\]❯\[\e[0m\] '
clear
