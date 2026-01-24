package YnMLang

import (
	"fmt"
)

func init() {
	fmt.Println(">> [YnMAdmin] English language loaded")
	RegisterLanguage("En", map[MessageKey]string{
	MsgInfoDisplay: "Nick: %s | Host: %s | Added by: %s | Lang: %s | MyChar: %s | Welcome: %s | Pass: %s | Role: %s | Email: %s | Created: %s | Channels: %s Console Channel: %s ",
	    MsgPasswordAccepted:          "🔐 Password accepted.",
		MsgPasswordUpdated:           "🔁 Password updated.",
		MsgPasswordTooShort:          "❌ Password must be 4-20 characters long.",
		MsgPasswordPrivateOnly:       "❌ Please set your password in private message only.",
		MsgIncorrectPassword:         "❌ Incorrect current password.",
		MsgNoPasswordSet:             "❌ No password is currently set. Use: %s",
		MsgPasswordUsage:             "ℹ️ Use: %s",
		MsgCharUpdated:               "Character updated to '%s'",
		MsgCharOneCharOnly:           "Error: 'char' must be exactly one character.",
		MsgWelcomeUpdated:            "Welcome message updated to '%s'",
		MsgLangUpdated:               "Language updated to '%s'",
		MsgInvalidLanguage:           "Error: Allowed languages are En, Ro, Hu (case sensitive).",
		MsgUnknownField:              "Unknown field. Allowed: char, welcome, pass, lang.",
		MsgUpdateError:               "Error updating %s: %v",
		MsgUsageSet:                  "Usage: %s",
		MsgAlreadyowner:              "You are already my owner!",
		MsgownerExists:               "There is already an owner registered. Only one owner is allowed.",
		MsgNewowner:                  "%s is now my owner! (Registered at %s)",
		MsgRegisterError:             "Error: Couldn't register owner",
		MsgNoInfo:                    "No info found for you.",
		MsgErrorFetching:             "Error fetching info.",
		MsgNotRegistered:             "You are not registered as an owner.",
		MsgPasswordNotSet:            "Not Set",
		MsgPasswordSet:               "✅",
		MsgNoPermission:              "You don't have permission to view other users' data.",

		MsgAutomodeChannelOnly:       "❌ Automode can only be set in channels.",
		MsgInvalidAutomode:           "❌ Invalid automode. Valid options: +v, +h, +o, off",
		MsgNoChannelAccess:           "❌ You don't have access to this channel.",
		MsgAutomodeInsufficientPerm:  "❌ Your permission level (%s) is insufficient for this automode.",
		MsgAutomodeUpdateError:       "❌ Error updating automode: %s",
		MsgAutomodeDisabled:          "✅ Automode disabled for %s",
		MsgAutomodeUpdated:           "✅ Automode set to %s for %s",
		MsgAutomodeUsage:             "Usage: %s",
		MsgBotNoPrivs:                "I don't have operator privileges in %s",
		MsgInvalidMode:               "Invalid mode command: %s",
		MsgModeApplied:               "%s: Applied modes %s to %s",
		MsgBotNoOpPrivilege:          "I don't have operator privileges in %s",
		MsgNoChannelModesImplemented: "Channel mode tracking is not implemented",
		MsgModeInfoChannelOnly:       "This command can only be used in channels",
		MsgErrorRetrievingSavedModes: "Error retrieving saved modes: %v",
		MsgNoSavedModes:              "No saved modes for this channel",
		MsgSavedModesInfo:            "Saved modes: %s (set by: %s, time: %s)",
		MsgNoChannelModesSaved:       "No channel modes currently saved",
		MsgErrorRetrievingChannelModes: "Error retrieving channel modes: %v",
		MsgSavedChannelModes:         "Saved channel modes: %s",
		MsgChannelModeHelp:           "Channel mode usage: !ynm set chmod <modes> [params], e.g. +k <key>, +l <limit>, +i (invite-only), +m (moderated), +n (no external messages), +t (topic protection)",
		MsgAccessNone:                "Access: <none>",
		MsgAutomodeNone:              "Automode: None",
		MsgNoUserInfo:                "No user information available.",
		MsgownerOnly:                 "Only users with owner permissions can use this command.",
		MsgRoomAdded:				"Channel: %s added",
		MsgAddRoomError:              "Failed to add channel: %s",
		MsgRoomCreated:               "Channel successfully created: %s",
		MsgRemoveRoomError:           "Failed to remove channel: %s",
		MsgRoomRemoved:               "Channel successfully removed: %s",
		MsgPassUpdated:               "Password successfully updated.",
		
		
		
MsgHostAdded       : "✅ Host %s successfully added.",
MsgHostDeleted     : "✅ Host %s successfully deleted.",
MsgHostAddError    : "❌ Error adding host: %s",
MsgHostDelError    : "❌ Error deleting host: %s",
	})
}