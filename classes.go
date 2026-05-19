package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include <stdlib.h>
#include "discord.h"
*/
import "C"
import (
	"runtime"
	"unsafe"
)

type ErrorType int

const (
	ErrorTypeNone                ErrorType = C.Discord_ErrorType_None
	ErrorTypeNetworkError        ErrorType = C.Discord_ErrorType_NetworkError
	ErrorTypeHTTPError           ErrorType = C.Discord_ErrorType_HTTPError
	ErrorTypeClientNotReady      ErrorType = C.Discord_ErrorType_ClientNotReady
	ErrorTypeDisabled             ErrorType = C.Discord_ErrorType_Disabled
	ErrorTypeClientDestroyed      ErrorType = C.Discord_ErrorType_ClientDestroyed
	ErrorTypeValidationError     ErrorType = C.Discord_ErrorType_ValidationError
	ErrorTypeAborted             ErrorType = C.Discord_ErrorType_Aborted
	ErrorTypeAuthorizationFailed ErrorType = C.Discord_ErrorType_AuthorizationFailed
	ErrorTypeRPCError            ErrorType = C.Discord_ErrorType_RPCError
)

type ActivityType int

const (
	ActivityTypePlaying      ActivityType = C.Discord_ActivityTypes_Playing
	ActivityTypeStreaming    ActivityType = C.Discord_ActivityTypes_Streaming
	ActivityTypeListening    ActivityType = C.Discord_ActivityTypes_Listening
	ActivityTypeWatching     ActivityType = C.Discord_ActivityTypes_Watching
	ActivityTypeCustomStatus ActivityType = C.Discord_ActivityTypes_CustomStatus
	ActivityTypeCompeting    ActivityType = C.Discord_ActivityTypes_Competing
	ActivityTypeHangStatus   ActivityType = C.Discord_ActivityTypes_HangStatus
)

type ChannelType int

const (
	ChannelTypeGuildText          ChannelType = C.Discord_ChannelType_GuildText
	ChannelTypeDm                 ChannelType = C.Discord_ChannelType_Dm
	ChannelTypeGuildVoice         ChannelType = C.Discord_ChannelType_GuildVoice
	ChannelTypeGroupDm            ChannelType = C.Discord_ChannelType_GroupDm
	ChannelTypeGuildCategory      ChannelType = C.Discord_ChannelType_GuildCategory
	ChannelTypeGuildNews          ChannelType = C.Discord_ChannelType_GuildNews
	ChannelTypeGuildStore         ChannelType = C.Discord_ChannelType_GuildStore
	ChannelTypeGuildNewsThread    ChannelType = C.Discord_ChannelType_GuildNewsThread
	ChannelTypeGuildPublicThread  ChannelType = C.Discord_ChannelType_GuildPublicThread
	ChannelTypeGuildPrivateThread ChannelType = C.Discord_ChannelType_GuildPrivateThread
	ChannelTypeGuildStageVoice    ChannelType = C.Discord_ChannelType_GuildStageVoice
	ChannelTypeGuildDirectory     ChannelType = C.Discord_ChannelType_GuildDirectory
	ChannelTypeGuildForum         ChannelType = C.Discord_ChannelType_GuildForum
	ChannelTypeGuildMedia         ChannelType = C.Discord_ChannelType_GuildMedia
	ChannelTypeLobby              ChannelType = C.Discord_ChannelType_Lobby
	ChannelTypeEphemeralDm        ChannelType = C.Discord_ChannelType_EphemeralDm
)

type StatusType int

const (
	StatusTypeOnline    StatusType = C.Discord_StatusType_Online
	StatusTypeOffline   StatusType = C.Discord_StatusType_Offline
	StatusTypeBlocked   StatusType = C.Discord_StatusType_Blocked
	StatusTypeIdle      StatusType = C.Discord_StatusType_Idle
	StatusTypeDnd       StatusType = C.Discord_StatusType_Dnd
	StatusTypeInvisible StatusType = C.Discord_StatusType_Invisible
	StatusTypeStreaming StatusType = C.Discord_StatusType_Streaming
	StatusTypeUnknown   StatusType = C.Discord_StatusType_Unknown
)

type RelationshipType int

const (
	RelationshipTypeNone            RelationshipType = C.Discord_RelationshipType_None
	RelationshipTypeFriend          RelationshipType = C.Discord_RelationshipType_Friend
	RelationshipTypeBlocked         RelationshipType = C.Discord_RelationshipType_Blocked
	RelationshipTypePendingIncoming RelationshipType = C.Discord_RelationshipType_PendingIncoming
	RelationshipTypePendingOutgoing RelationshipType = C.Discord_RelationshipType_PendingOutgoing
	RelationshipTypeImplicit        RelationshipType = C.Discord_RelationshipType_Implicit
	RelationshipTypeSuggestion      RelationshipType = C.Discord_RelationshipType_Suggestion
)

type LoggingSeverity int

const (
	LoggingSeverityVerbose LoggingSeverity = C.Discord_LoggingSeverity_Verbose
	LoggingSeverityInfo    LoggingSeverity = C.Discord_LoggingSeverity_Info
	LoggingSeverityWarning LoggingSeverity = C.Discord_LoggingSeverity_Warning
	LoggingSeverityError   LoggingSeverity = C.Discord_LoggingSeverity_Error
	LoggingSeverityNone    LoggingSeverity = C.Discord_LoggingSeverity_None
)

// Helper to convert Go string to Discord_String
func toDiscordString(s string) C.Discord_String {
	if s == "" {
		return C.Discord_String{ptr: nil, size: 0}
	}
	return C.Discord_String{
		ptr:  (*C.uint8_t)(unsafe.Pointer(C.CString(s))),
		size: C.size_t(len(s)),
	}
}

// Helper to free C string allocated by toDiscordString
func freeDiscordString(s C.Discord_String) {
	if s.ptr != nil {
		C.free(unsafe.Pointer(s.ptr))
	}
}

// Helper to convert Discord_String to Go string
func fromDiscordString(s C.Discord_String) string {
	if s.ptr == nil || s.size == 0 {
		return ""
	}
	return C.GoStringN((*C.char)(unsafe.Pointer(s.ptr)), C.int(s.size))
}

type Activity struct {
	c C.struct_Discord_Activity
}

func NewActivity() *Activity {
	a := &Activity{}
	C.Discord_Activity_Init(&a.c)
	runtime.SetFinalizer(a, func(a *Activity) {
		C.Discord_Activity_Drop(&a.c)
	})
	return a
}

func (a *Activity) SetName(name string) {
	s := toDiscordString(name)
	defer freeDiscordString(s)
	C.Discord_Activity_SetName(&a.c, s)
}

func (a *Activity) Name() string {
	var s C.Discord_String
	C.Discord_Activity_Name(&a.c, &s)
	return fromDiscordString(s)
}

func (a *Activity) SetType(t ActivityType) {
	C.Discord_Activity_SetType(&a.c, C.Discord_ActivityTypes(t))
}

func (a *Activity) Type() ActivityType {
	return ActivityType(C.Discord_Activity_Type(&a.c))
}

func (a *Activity) SetState(state string) {
	s := toDiscordString(state)
	defer freeDiscordString(s)
	C.Discord_Activity_SetState(&a.c, &s)
}

func (a *Activity) State() string {
	var s C.Discord_String
	if bool(C.Discord_Activity_State(&a.c, &s)) {
		return fromDiscordString(s)
	}
	return ""
}

func (a *Activity) SetDetails(details string) {
	s := toDiscordString(details)
	defer freeDiscordString(s)
	C.Discord_Activity_SetDetails(&a.c, &s)
}

func (a *Activity) Details() string {
	var s C.Discord_String
	if bool(C.Discord_Activity_Details(&a.c, &s)) {
		return fromDiscordString(s)
	}
	return ""
}

type ActivityAssets struct {
	c C.struct_Discord_ActivityAssets
}

func NewActivityAssets() *ActivityAssets {
	a := &ActivityAssets{}
	C.Discord_ActivityAssets_Init(&a.c)
	runtime.SetFinalizer(a, func(a *ActivityAssets) {
		C.Discord_ActivityAssets_Drop(&a.c)
	})
	return a
}

func (a *ActivityAssets) SetLargeImage(image string) {
	s := toDiscordString(image)
	defer freeDiscordString(s)
	C.Discord_ActivityAssets_SetLargeImage(&a.c, &s)
}

func (a *ActivityAssets) LargeImage() string {
	var s C.Discord_String
	if bool(C.Discord_ActivityAssets_LargeImage(&a.c, &s)) {
		return fromDiscordString(s)
	}
	return ""
}

func (a *Activity) SetAssets(assets *ActivityAssets) {
	if assets == nil {
		C.Discord_Activity_SetAssets(&a.c, nil)
	} else {
		C.Discord_Activity_SetAssets(&a.c, &assets.c)
	}
}

func (a *Activity) Assets() *ActivityAssets {
	assets := &ActivityAssets{}
	C.Discord_ActivityAssets_Init(&assets.c)
	if bool(C.Discord_Activity_Assets(&a.c, &assets.c)) {
		runtime.SetFinalizer(assets, func(a *ActivityAssets) {
			C.Discord_ActivityAssets_Drop(&a.c)
		})
		return assets
	}
	return nil
}

type ClientResult struct {
	c C.struct_Discord_ClientResult
}

func (r *ClientResult) Type() ErrorType {
	return ErrorType(C.Discord_ClientResult_Type(&r.c))
}

func (r *ClientResult) Successful() bool {
	return bool(C.Discord_ClientResult_Successful(&r.c))
}

func (r *ClientResult) Error() string {
	var s C.Discord_String
	C.Discord_ClientResult_Error(&r.c, &s)
	return fromDiscordString(s)
}

type User struct {
	c C.struct_Discord_UserHandle
}

func (u *User) ID() uint64 {
	return uint64(C.Discord_UserHandle_Id(&u.c))
}

func (u *User) Username() string {
	var s C.Discord_String
	C.Discord_UserHandle_Username(&u.c, &s)
	return fromDiscordString(s)
}

func (u *User) DisplayName() string {
	var s C.Discord_String
	C.Discord_UserHandle_DisplayName(&u.c, &s)
	return fromDiscordString(s)
}

type Relationship struct {
	c C.struct_Discord_RelationshipHandle
}

func (r *Relationship) Type() RelationshipType {
	return RelationshipType(C.Discord_RelationshipHandle_DiscordRelationshipType(&r.c))
}

func (r *Relationship) User() *User {
	u := &User{}
	if bool(C.Discord_RelationshipHandle_User(&r.c, &u.c)) {
		runtime.SetFinalizer(u, func(u *User) {
			C.Discord_UserHandle_Drop(&u.c)
		})
		return u
	}
	return nil
}

type ClientCreateOptions struct {
	c C.struct_Discord_ClientCreateOptions
}

func NewClientCreateOptions() *ClientCreateOptions {
	o := &ClientCreateOptions{}
	C.Discord_ClientCreateOptions_Init(&o.c)
	runtime.SetFinalizer(o, func(o *ClientCreateOptions) {
		C.Discord_ClientCreateOptions_Drop(&o.c)
	})
	return o
}

func (o *ClientCreateOptions) SetWebBase(webBase string) {
	s := toDiscordString(webBase)
	defer freeDiscordString(s)
	C.Discord_ClientCreateOptions_SetWebBase(&o.c, s)
}

func (o *ClientCreateOptions) SetApiBase(apiBase string) {
	s := toDiscordString(apiBase)
	defer freeDiscordString(s)
	C.Discord_ClientCreateOptions_SetApiBase(&o.c, s)
}

type Lobby struct {
	c C.struct_Discord_LobbyHandle
}

type LobbyMember struct {
	c C.struct_Discord_LobbyMemberHandle
}

type Message struct {
	c C.struct_Discord_MessageHandle
}

type Channel struct {
	c C.struct_Discord_ChannelHandle
}

type GuildMinimal struct {
	c C.struct_Discord_GuildMinimal
}

type GuildChannel struct {
	c C.struct_Discord_GuildChannel
}

type Call struct {
	c C.struct_Discord_Call
}

type AudioDevice struct {
	c C.struct_Discord_AudioDevice
}

type UserMessageSummary struct {
	c C.struct_Discord_UserMessageSummary
}
