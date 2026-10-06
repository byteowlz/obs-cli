-- Native name/screen picker around the bundled obs-cli. No credentials are embedded.
on reopen
    my controlRecording()
end reopen

on run
    my controlRecording()
end run

on controlRecording()
    try
        set appPath to POSIX path of (path to me)
        if appPath does not end with "/" then set appPath to appPath & "/"
        set resources to appPath & "Contents/Resources/"
        set cli to resources & "obs-cli"
        set displayHelper to resources & "obs-recording-displays"
        do shell script "/usr/bin/test -x " & quoted form of cli & " && /usr/bin/test -x " & quoted form of displayHelper
        set stateRoot to do shell script "/usr/bin/printf '%s' \"${XDG_STATE_HOME:-$HOME/.local/state}\""
        if stateRoot does not start with "/" then error "XDG_STATE_HOME must be an absolute path."
        set stateDir to stateRoot & "/obs-cli"
        set stateFile to stateDir & "/last-gui-session.txt"
        set screenFile to stateDir & "/last-screen.txt"
        set statusText to my recordingStatus(cli)
        if statusText starts with "Recording: true" then
            display dialog "Stop this recording and finish the three MKV outputs?" with title "OBS Recording" buttons {"Cancel", "Stop Recording"} default button "Stop Recording" cancel button "Cancel"
            do shell script quoted form of cli & " recording stop"
            set answer to display dialog "Recording stopped." with title "OBS Recording" buttons {"Done", "Show Last Folder"} default button "Done"
            if button returned of answer is "Show Last Folder" then
                set folderPath to do shell script "/bin/cat " & quoted form of stateFile
                do shell script "/usr/bin/open " & quoted form of folderPath
            end if
        else
            set streamText to do shell script quoted form of cli & " stream status"
            if streamText starts with "Streaming: true" then error "Stop streaming before starting a named recording."
            set screenChoice to my pickScreen(displayHelper, screenFile)
            if screenChoice is false then return
            set selectedUUID to item 1 of screenChoice
            set selectedLabel to item 2 of screenChoice
            set answer to display dialog "Recording name for:\n" & selectedLabel & "\n\nAll three MKVs go to the configured dated folder. The composite follows the selected screen unless configured otherwise.\n\nSelect your configured OBS profile, collection and scene (defaults: MultiTrack / Composite)." with title "OBS Recording" default answer "" buttons {"Cancel", "Start Recording"} default button "Start Recording" cancel button "Cancel"
            set recordingName to text returned of answer
            set folderPath to do shell script quoted form of cli & " recording session " & quoted form of recordingName & " --screen " & quoted form of selectedUUID
            do shell script "/bin/mkdir -p " & quoted form of stateDir & " && /usr/bin/printf '%s\\n' " & quoted form of folderPath & " > " & quoted form of stateFile & " && /usr/bin/printf '%s\\n' " & quoted form of selectedUUID & " > " & quoted form of screenFile
            display notification "Saved to " & folderPath with title "OBS Recording Started"
        end if
    on error errorText number errorNumber
        if errorNumber is not -128 then
            display alert "OBS recording could not complete" message errorText as warning
        end if
    end try
end controlRecording

on recordingStatus(cli)
    set launchedOBS to false
    try
        do shell script "/usr/bin/pgrep -x OBS >/dev/null"
    on error
        do shell script "/usr/bin/open -a OBS"
        set launchedOBS to true
    end try
    repeat with attempt from 1 to 20
        try
            return do shell script quoted form of cli & " recording status"
        on error errorText
            if not launchedOBS or attempt is 20 then error errorText
            delay 1
        end try
    end repeat
end recordingStatus

on pickScreen(helperPath, statePath)
    set displayRows to paragraphs of (do shell script quoted form of helperPath)
    set labels to {}
    set uuids to {}
    set lastScreen to ""
    try
        set lastScreen to do shell script "/bin/cat " & quoted form of statePath
    end try
    set defaultIndex to 1
    repeat with rowText in displayRows
        if (rowText as text) is not "" then
            set previousDelimiters to AppleScript's text item delimiters
            set AppleScript's text item delimiters to ASCII character 9
            set fields to text items of (rowText as text)
            set AppleScript's text item delimiters to previousDelimiters
            if (count fields) is not 2 then error "Invalid display-discovery result."
            set end of uuids to item 1 of fields
            set end of labels to ((count labels) + 1) & ". " & item 2 of fields as text
            if (item 1 of fields) is lastScreen then set defaultIndex to count labels
        end if
    end repeat
    if (count labels) is 0 then error "No active screens are available."
    set selection to choose from list labels with title "OBS Recording - Screen" with prompt "Select the screen to record. The configured composition mode will be applied." default items {item defaultIndex of labels} OK button name "Use Screen" cancel button name "Cancel"
    if selection is false then return false
    repeat with i from 1 to count labels
        if (item i of labels) is (item 1 of selection) then return {item i of uuids, item i of labels}
    end repeat
    error "No screen was selected."
end pickScreen
