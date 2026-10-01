Output demo/assets/terminal.gif

Set Shell bash
Set FontSize 17
Set Width 1180
Set Height 640
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms
Set Framerate 24

Hide
Type "export PS1='eng $ ' ENG_ROOT=$ENG_DEMO_ROOT PATH=$PWD/bin:$PATH"
Enter
Type "clear"
Enter
Show

Type "# a prompt written in Claude Code -> corrections at the end of the turn"
Enter
Sleep 600ms
Type `echo '{"session_id":"demo"}' | eng hook-stop | jq -r .systemMessage | fold -s -w 104`
Enter
Sleep 6s

Type "clear"
Enter
Type "# which topics are ripe for a drill"
Enter
Type "eng status"
Enter
Sleep 5s

Type "clear"
Enter
Type "# drill cards are cut from your own phrases"
Enter
Type "eng drill next -topic the -n 3 | jq -c '.cards[] | {type, text, choices}'"
Enter
Sleep 5s
