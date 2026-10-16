# Records demo/demo.gif. Run from the repository root: vhs demo/demo.tape
Output demo/demo.gif

Require go

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 860
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type "go build -o /tmp/worlds-tui-demo ./cmd/worlds-tui && clear"
Enter
Sleep 10s
Show

Type "worlds-tui -tournament worlds_2025"
Sleep 500ms
Hide
Ctrl+U
Type "/tmp/worlds-tui-demo -tournament worlds_2025"
Enter
Show
Sleep 5s

# Menu: walk down the sections, then open Live
Down
Sleep 700ms
Down
Sleep 700ms
Down
Sleep 700ms
Up
Sleep 700ms
Up
Sleep 700ms
Up
Sleep 700ms
Enter
Sleep 5s
Down@200ms 6
Sleep 1500ms
Up@200ms 6
Sleep 1s

# Play-In bracket
Type "2"
Sleep 3s

# Swiss Stage (tab moves to the next view): rounds, records and standings
Tab
Sleep 2s
Down@400ms 4
Sleep 2s

# Knockouts and Schedule
Type "4"
Sleep 3s
Type "5"
Sleep 3s

# Back to the menu and quit
Escape
Sleep 2s
Type "q"
Sleep 1s
