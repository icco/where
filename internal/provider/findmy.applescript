-- Read native Accessibility labels; Foundation is used only to encode JSON.
use framework "Foundation"
use scripting additions

tell application id "com.apple.findmy" to activate
delay 1
tell application "System Events"
	if not UI elements enabled then error "Accessibility permission required"
	tell process "FindMy"
		set frontmost to true
		set foundPeople to false
		repeat with elem in (entire contents of window 1)
			try
				if class of elem is radio button and (name of elem is "People" or description of elem is "People") then
					click elem
					set foundPeople to true
					exit repeat
				end if
			end try
		end repeat
		if not foundPeople then error "Cannot identify People tab"
		delay 1
		set records to current application's NSMutableArray's array()
		set titleCount to 0
		repeat with elem in (entire contents of window 1)
			set axID to ""
			try
				set axID to value of attribute "AXIdentifier" of elem
			end try
			if axID is "HomeCellTitleLabel" then
				set titleCount to titleCount + 1
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
					records's addObject:recordValue
				end if
			end if
		end repeat
		-- An unknown layout must not masquerade as a successful empty list.
		if titleCount is 0 then error "No recognizable People rows; check sign-in and layout"
	end tell
end tell
set encoded to current application's NSJSONSerialization's dataWithJSONObject:records options:0 |error|:(missing value)
set resultText to current application's NSString's alloc()'s initWithData:encoded encoding:(current application's NSUTF8StringEncoding)
return resultText as text
