-- Uses the existing signed-in session and native Accessibility labels.
-- Read pages top-to-bottom to include virtualized/offscreen People rows.
use framework "Foundation"
use scripting additions

on readPage()
	set outputRows to current application's NSMutableArray's array()
	tell application "System Events" to tell process "FindMy"
		repeat with elem in (entire contents of window 1)
			set axID to ""
			try
				set axID to value of attribute "AXIdentifier" of elem
			end try
			if axID is "HomeCellTitleLabel" then
				set personName to description of elem
				if personName is not "Me" then
					set placeText to ""
					set statusText to ""
					set rowElement to value of attribute "AXParent" of elem
					repeat with labelElement in (entire contents of rowElement)
						try
							set labelID to value of attribute "AXIdentifier" of labelElement
							if labelID is "HomeCellSubtitleLabel" then set placeText to description of labelElement
							if labelID is "HomeCellDetailLabel" then set statusText to description of labelElement
						end try
					end repeat
					set recordValue to current application's NSMutableDictionary's dictionary()
					recordValue's setObject:personName forKey:"name"
					recordValue's setObject:placeText forKey:"location"
					recordValue's setObject:statusText forKey:"status"
					outputRows's addObject:recordValue
				end if
			end if
		end repeat
	end tell
	return outputRows
end readPage

tell application "System Events"
	if not UI elements enabled then error "Accessibility permission required"
end tell
tell application id "com.apple.findmy" to activate
delay 1
set pageList to current application's NSMutableArray's array()
tell application "System Events"
	tell process "FindMy"
		set frontmost to true
		-- The View menu survives the macOS 26 floating-sidebar redesign.
		click menu item "People" of menu "View" of menu bar 1
		delay 1
		set firstTitle to missing value
		repeat with elem in (entire contents of window 1)
			try
				if value of attribute "AXIdentifier" of elem is "HomeCellTitleLabel" then
					set firstTitle to contents of elem
					exit repeat
				end if
			end try
		end repeat
		-- Unknown layouts and signed-out screens must not become empty successes.
		if firstTitle is missing value then error "No recognizable People rows"
		set ancestor to firstTitle
		set sidebar to missing value
		repeat 30 times
			set ancestor to value of attribute "AXParent" of ancestor
			if class of ancestor is scroll area then
				set sidebar to ancestor
				exit repeat
			end if
		end repeat
		if sidebar is missing value then error "Cannot identify People scroll area"
		set bars to every scroll bar of sidebar
		if (count of bars) is 0 then
			pageList's addObject:(my readPage())
		else
			set verticalBar to missing value
			repeat with barElement in bars
				if value of attribute "AXOrientation" of barElement is "AXVerticalOrientation" then set verticalBar to contents of barElement
			end repeat
			if verticalBar is missing value then error "Cannot identify vertical scroll bar"
			set value of verticalBar to 0
			delay 0.3
			pageList's addObject:(my readPage())
			-- Small overlapping steps; Go checks overlap before accepting each page.
			repeat with stepNumber from 1 to 20
				set targetValue to stepNumber / 20
				set value of verticalBar to targetValue
				delay 0.2
				if (value of verticalBar as real) < targetValue - 0.02 then error "People scrolling failed"
				pageList's addObject:(my readPage())
			end repeat
		end if
	end tell
end tell
set envelope to current application's NSMutableDictionary's dictionary()
envelope's setObject:pageList forKey:"pages"
set encoded to (current application's NSJSONSerialization's dataWithJSONObject:envelope options:0 |error|:(missing value))
set resultText to (current application's NSString's alloc()'s initWithData:encoded encoding:(current application's NSUTF8StringEncoding))
return resultText as text
