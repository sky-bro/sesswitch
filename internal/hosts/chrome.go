package hosts

import "context"

const focusChromeTabScript = `on run argv
set targetTabID to item 1 of argv
set targetURL to item 2 of argv
tell application "Google Chrome"
  set allTabIDs to id of every tab of every window
  repeat with windowIndex from 1 to count allTabIDs
    set windowTabIDs to item windowIndex of allTabIDs
    repeat with tabIndex from 1 to count windowTabIDs
      if (item tabIndex of windowTabIDs as text) is targetTabID then
        set active tab index of window windowIndex to tabIndex
        set index of window windowIndex to 1
        activate
        return
      end if
    end repeat
  end repeat
  if targetURL is not "" then
    set matchCount to 0
    set matchedWindow to 0
    set matchedTab to 0
    set allTabURLs to URL of every tab of every window
    repeat with windowIndex from 1 to count allTabURLs
      set windowTabURLs to item windowIndex of allTabURLs
      repeat with tabIndex from 1 to count windowTabURLs
        if item tabIndex of windowTabURLs is targetURL then
          set matchCount to matchCount + 1
          set matchedWindow to windowIndex
          set matchedTab to tabIndex
        end if
      end repeat
    end repeat
    if matchCount is 1 then
      set active tab index of window matchedWindow to matchedTab
      set index of window matchedWindow to 1
      activate
      return
    end if
  end if
end tell
error "Chrome tab is unavailable or ambiguous" number 2
end run`

// FocusChromeTab prefers a stable tab ID, then an unambiguous exact URL.
// Browser context is passed as arguments, never interpolated into AppleScript.
func FocusChromeTab(ctx context.Context, tabID, tabURL string, run Runner) error {
	return run(ctx, "osascript", "-e", focusChromeTabScript, "--", tabID, tabURL)
}
