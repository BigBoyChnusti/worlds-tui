# Records demo/demo.gif. Run from the repository root, either with VHS
# installed (it builds the app if ./worlds-tui is missing):
#
#   vhs demo/demo.tape
#
# or with the VHS container image, which has no Go, after building:
#
#   CGO_ENABLED=0 go build -o worlds-tui ./cmd/worlds-tui
#   podman run --rm -v "$PWD:/vhs:Z" ghcr.io/charmbracelet/vhs:v0.12.1 demo/demo.tape
Output demo/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 860
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set Framerate 20

Hide
Type "test -x ./worlds-tui || go build -o worlds-tui ./cmd/worlds-tui; clear"
Enter
Sleep 15s
Show

Type "worlds-tui -tournament worlds_2025"
Sleep 500ms
Hide
Ctrl+U
Type "./worlds-tui -tournament worlds_2025"
Enter
Show
Sleep 5s

# Menu
Down@600ms 2
Up@600ms 2
Sleep 500ms

# Swiss Stage: up/down move within a round, left/right between rounds
Type "3"
Sleep 2s
Down@450ms 3
Right@600ms 2
Down@450ms 2
Left@600ms 1
Up@450ms 2
Sleep 1s

# Knockouts: arrow over to the final and open it
Type "4"
Sleep 2s
Down@500ms 1
Right@700ms 2
Sleep 1s
Enter
Sleep 8s

# The final on the Live tab: scoreboard and gold chart, then game 4 and back
Left
Sleep 5s
Right
Sleep 3s

# Page down to the player tables with champions and items (two full
# redraws instead of one per line keeps the GIF small)
PageDown@800ms 2
Sleep 4s

# Schedule
Type "5"
Sleep 2s
Down@350ms 4
Sleep 1500ms

# Back to the menu, then quit off camera so the GIF ends on the menu
Escape
Sleep 3s
Hide
Type "q"
Sleep 1s
