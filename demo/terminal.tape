Output demo/assets/terminal.gif

Set Shell bash
Set FontSize 20
Set Width 960
Set Height 420
Set Padding 28
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms
Set Framerate 24

Hide
Type `export PS1='$ ' ENG_ROOT=$ENG_DEMO_ROOT PATH=$PWD/bin:$PATH`
Enter
Type `eng() { if [ "$1" = hook-stop ]; then echo '{"session_id":"demo"}' | command eng hook-stop | jq -r .systemMessage | grep -v 'last week' | sed -E 's# [0-9]+/[0-9]+##g' | sed 's/ · /\n   /g; s/): /):\n   /' | sed -E 's/ [(][a-z-]+[)]$//; \#^   (/eng-drill|eng open)#d' | tr -d '\140'; else command eng "$@"; fi; }`
Enter
Type "clear"
Enter
Show

Type "# 1. you write in Claude Code, eng catches the mistakes"
Enter
Sleep 400ms
Type "eng hook-stop"
Enter
Sleep 5s

Type "clear"
Enter
Type "# 2. topics ripen from your own mistakes"
Enter
Sleep 400ms
Type "eng status"
Enter
Sleep 4s
