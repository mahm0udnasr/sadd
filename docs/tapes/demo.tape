# Main demo for the README: a tour of the interactive screen (TUI).
# Record from the repo root:  make && vhs docs/tapes/demo.tape
Output docs/images/demo.gif
Source docs/tapes/common.tape
Set Height 760

# Start with a few sites already blocked, off camera.
Hide
Type "source docs/tapes/setup.sh" Enter
Type "sudo sadd block youtube.com --for 1h30m >/dev/null" Enter
Type "sudo sadd block reddit.com x.com instagram.com tiktok.com >/dev/null; clear" Enter
Sleep 1s
Show

Type "sudo sadd" Sleep 500ms Enter
Sleep 2s

# Details of a timed site, then extend its timer by one hour.
Enter
Sleep 2s
Down Sleep 600ms
Enter
Sleep 1.2s
Right Sleep 900ms
Enter
Sleep 2s

# Add sites: full URLs are fine, the preview shows what will be blocked.
Type "a"
Sleep 800ms
Type "https://www.twitch.tv/directory netflix.com"
Sleep 1.5s
Enter
Sleep 800ms
Right Sleep 600ms Right Sleep 600ms Right Sleep 600ms
Enter
Sleep 2s

# Select two sites with space and unblock them together.
Down Sleep 500ms
Space Sleep 500ms Down Sleep 500ms Space
Sleep 1s
Type "d"
Sleep 1.5s
Type "y"
Sleep 2s

# Groups tab: block the whole "shopping" group for one hour.
Tab
Sleep 1.5s
Down Sleep 500ms Down Sleep 500ms
Enter
Sleep 1.5s
Down Sleep 500ms Down Sleep 500ms Down Sleep 500ms
Enter
Sleep 2s

# Back to the list to see the result, then quit.
Tab
Sleep 2.5s
Type "q"
Sleep 800ms
