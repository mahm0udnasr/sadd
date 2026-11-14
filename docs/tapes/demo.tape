# Main demo for the README: CLI basics, then the interactive screen.
# Record from the repo root:  make && vhs docs/tapes/demo.tape
Output docs/images/demo.gif
Source docs/tapes/common.tape
Set Height 720

Hide
Type "source docs/tapes/setup.sh" Enter
Sleep 500ms
Show

Type "sudo sadd block https://www.YouTube.com/watch?v=dQw4 --for 2h" Sleep 400ms Enter
Sleep 1.5s
Type "sudo sadd block --group social" Sleep 400ms Enter
Sleep 1.5s
Type "clear && sadd list" Sleep 400ms Enter
Sleep 2.5s
Type "sadd check youtube.com facebook.com netflix.com" Sleep 400ms Enter
Sleep 3s

Type "clear && sudo sadd" Sleep 400ms Enter
Sleep 2s
Down Sleep 400ms Down Sleep 400ms
Space Sleep 400ms Down Sleep 400ms Space
Sleep 1.2s
Type "d"
Sleep 1.5s
Type "y"
Sleep 1.5s
Type "a"
Sleep 800ms
Type "netflix.com twitch.tv"
Sleep 1.2s
Enter
Sleep 1s
Right Sleep 700ms Right Sleep 700ms
Enter
Sleep 2s
Tab
Sleep 2s
Tab
Sleep 1s
Type "q"
Sleep 800ms
