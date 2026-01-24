package YnMLang

import (

)

// MessageKey represents different message types
type MessageKey string

const (
	MsgInfoDisplay                MessageKey = "info_display"
	MsgPasswordAccepted          MessageKey = "password_accepted"
	MsgPasswordUpdated           MessageKey = "password_updated"
	MsgPasswordTooShort          MessageKey = "password_too_short"
	MsgPasswordPrivateOnly       MessageKey = "password_private_only"
	MsgIncorrectPassword         MessageKey = "incorrect_password"
	MsgNoPasswordSet             MessageKey = "no_password_set"
	MsgPasswordUsage             MessageKey = "password_usage"
	MsgCharUpdated               MessageKey = "char_updated"
	MsgCharOneCharOnly           MessageKey = "char_one_char_only"
	MsgWelcomeUpdated            MessageKey = "welcome_updated"
	MsgLangUpdated               MessageKey = "lang_updated"
	MsgInvalidLanguage           MessageKey = "invalid_language"
	MsgUnknownField              MessageKey = "unknown_field"
	MsgUpdateError               MessageKey = "update_error"
	MsgUsageSet                  MessageKey = "usage_set"
	MsgAlreadyowner              MessageKey = "already_owner"
	MsgownerExists               MessageKey = "owner_exists"
	MsgNewowner                  MessageKey = "new_owner"
	MsgRegisterError             MessageKey = "register_error"
	MsgNoInfo                    MessageKey = "no_info"
	MsgErrorFetching             MessageKey = "error_fetching"
	MsgNotRegistered             MessageKey = "not_registered"
	MsgPasswordNotSet            MessageKey = "password_not_set"
	MsgPasswordSet               MessageKey = "password_set"
	MsgNoPermission              MessageKey = "no_permission"
	MsgInfoDisplayWithAccess     MessageKey = "info_display_with_access"
	MsgAutomodeChannelOnly       MessageKey = "automode_channel_only"
	MsgInvalidAutomode           MessageKey = "invalid_automode"
	MsgNoChannelAccess           MessageKey = "no_channel_access"
	MsgAutomodeInsufficientPerm  MessageKey = "automode_insufficient_permission"
	MsgAutomodeUpdateError       MessageKey = "automode_update_error"
	MsgAutomodeDisabled          MessageKey = "automode_disabled"
	MsgAutomodeUpdated           MessageKey = "automode_updated"
	MsgAutomodeUsage             MessageKey = "automode_usage"
	MsgBotNoPrivs   MessageKey = "bot_no_privileges"
	MsgInvalidMode  MessageKey = "invalid_mode"
	MsgModeApplied  MessageKey = "mode_applied"
	MsgBotNoOpPrivilege       MessageKey = "bot_no_op_privilege"
	MsgNoChannelModesImplemented MessageKey = "no_channel_modes_implemented"
	MsgModeInfoChannelOnly    MessageKey = "mode_info_channel_only"
	MsgErrorRetrievingSavedModes MessageKey = "error_retrieving_saved_modes"
	MsgNoSavedModes           MessageKey = "no_saved_modes"
	MsgSavedModesInfo         MessageKey = "saved_modes_info"
	MsgNoChannelModesSaved    MessageKey = "no_channel_modes_saved"
	MsgErrorRetrievingChannelModes MessageKey = "error_retrieving_channel_modes"
	MsgSavedChannelModes      MessageKey = "saved_channel_modes"
	MsgChannelModeHelp        MessageKey = "channel_mode_help"
    MsgAccessNone                MessageKey = "access_none"
    MsgAutomodeNone              MessageKey = "automode_none"
    MsgNoUserInfo                MessageKey = "no_user_info"
    MsgownerOnly                 MessageKey = "owner_only"
    MsgAddRoomError              MessageKey = "add_room_error"
	MsgRoomAdded					MessageKey = "add_room_added"		
    MsgRoomCreated               MessageKey = "room_created"
    MsgRemoveRoomError           MessageKey = "remove_room_error"
    MsgRoomRemoved               MessageKey = "room_removed"
	MsgPassUpdated MessageKey = "pass_updated"
	MsgInvalidChannelName MessageKey = "invalid_channel_name"
	MsgDatabaseError MessageKey = "database_error"
	MsgChannelAlreadyExists	MessageKey = "channel_already_exist"
	MsgChannelNotFound	MessageKey = "channel_not_found"
	MsgInvalidHostmask	MessageKey = "invalid_hostmask"
	MsgInvalidRole	MessageKey = "invalid_role"
	MsgAddUserToChannelError	MessageKey = "adduser_to_chann_error"
	MsgUserAddedToChannel	MessageKey = "useradd_to_chann_error"
	MsgAddUserError 			MessageKey = "adduser_error"
	MsgAccessDenied	MessageKey = "access_denied"
	
MsgHostAdded       MessageKey = "host_added"
MsgHostDeleted     MessageKey = "host_deleted"
MsgHostAddError    MessageKey = "host_add_error"
MsgHostDelError    MessageKey = "host_del_error"

	)

// Messages contains all translations - will be populated by language files
var Messages = make(map[string]map[MessageKey]string)

// RegisterLanguage registers a new language with its translations
func RegisterLanguage(langCode string, translations map[MessageKey]string) {
	Messages[langCode] = translations
}

