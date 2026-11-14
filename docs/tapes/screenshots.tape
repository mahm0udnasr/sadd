# Still images for the README. Record from the repo root:
#   make && vhs docs/tapes/screenshots.tape
# VHS requires an Output, but only the PNGs are kept (the GIF is deleted
# by hand afterwards; it is in .gitignore).
Output docs/tapes/screenshots.gif
Source docs/tapes/common.tape
Set Height 780
Set TypingSpeed 10ms

Hide
Type "source docs/tapes/setup.sh" Enter
Type "sudo sadd block youtube.com reddit.com --for 2h >/dev/null" Enter
Type "sudo sadd block --group social >/dev/null; sudo sadd block netflix.com --for 45m >/dev/null; clear" Enter
Sleep 1s
Show

# CLI: list and check
Type "sadd list" Enter
Sleep 800ms
Type "sadd check youtube.com netflix.com amazon.com" Enter
Sleep 1.5s
Screenshot docs/images/cli.png
Sleep 1s

# TUI: main screen
Type "clear && sudo sadd" Enter
Sleep 2s
Screenshot docs/images/tui-blocked.png
Sleep 1s

# TUI: details dialog for youtube.com
Enter
Sleep 1.5s
Screenshot docs/images/tui-details.png
Sleep 1s

# TUI: groups tab
Escape Sleep 800ms
Tab
Sleep 1.5s
Screenshot docs/images/tui-groups.png
Sleep 1s

# TUI: add sites dialog with live preview (last: it ends with a dialog open)
Type "a" Sleep 800ms
Type "https://www.twitch.tv/directory news.ycombinator.com"
Sleep 1.5s
Screenshot docs/images/tui-add.png
Sleep 1s

Ctrl+C
Sleep 300ms
