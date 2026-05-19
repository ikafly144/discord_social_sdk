package sdk

/*
#include "discord.h"

void updateRichPresence_c(Discord_ClientResult* result, void* userData);
void activityInvite_c(Discord_ActivityInvite* invite, void* userData);
void activityJoin_c(Discord_String joinSecret, void* userData);
void acceptActivityInvite_c(Discord_ClientResult* result, Discord_String joinSecret, void* userData);
void simple_c(Discord_ClientResult* result, void* userData);
void freeUserData_c(void* userData);
*/
import "C"
import (
	"runtime"
	"unsafe"
)

//export goAcceptActivityInviteCallback
func goAcceptActivityInviteCallback(result *C.Discord_ClientResult, joinSecret C.Discord_String, id uintptr) {
	cb := getCallback(id).(func(ErrorType, string))
	cb(ErrorType(C.Discord_ClientResult_Type(result)), fromDiscordString(joinSecret))
}

//export goActivityInviteCallback
func goActivityInviteCallback(invite *C.struct_Discord_ActivityInvite, id uintptr) {
	cb := getCallback(id).(func(*ActivityInvite))
	i := &ActivityInvite{}
	C.Discord_ActivityInvite_Clone(&i.c, invite)
	runtime.SetFinalizer(i, func(i *ActivityInvite) {
		C.Discord_ActivityInvite_Drop(&i.c)
	})
	cb(i)
}

//export goActivityJoinCallback
func goActivityJoinCallback(joinSecret C.Discord_String, id uintptr) {
	cb := getCallback(id).(func(string))
	cb(fromDiscordString(joinSecret))
}

func (c *Client) UpdateRichPresence(activity *Activity, callback func(ErrorType)) {
	id := registerCallback(callback)
	C.Discord_Client_UpdateRichPresence(
		&c.cclient,
		&activity.c,
		(C.Discord_Client_UpdateRichPresenceCallback)(C.updateRichPresence_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) ClearRichPresence() {
	C.Discord_Client_ClearRichPresence(&c.cclient)
}

func (c *Client) SetActivityJoinCallback(callback func(string)) {
	id := registerCallback(callback)
	C.Discord_Client_SetActivityJoinCallback(
		&c.cclient,
		(C.Discord_Client_ActivityJoinCallback)(C.activityJoin_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SetActivityInviteCreatedCallback(callback func(*ActivityInvite)) {
	id := registerCallback(callback)
	C.Discord_Client_SetActivityInviteCreatedCallback(
		&c.cclient,
		(C.Discord_Client_ActivityInviteCallback)(C.activityInvite_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) AcceptActivityInvite(invite *ActivityInvite, callback func(ErrorType, string)) {
	id := registerCallback(callback)
	C.Discord_Client_AcceptActivityInvite(
		&c.cclient,
		&invite.c,
		(C.Discord_Client_AcceptActivityInviteCallback)(C.acceptActivityInvite_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SendActivityInvite(userID uint64, content string, callback func(ErrorType)) {
	id := registerCallback(callback)
	cContent := toDiscordString(content)
	defer freeDiscordString(cContent)

	C.Discord_Client_SendActivityInvite(
		&c.cclient,
		C.uint64_t(userID),
		cContent,
		(C.Discord_Client_SendActivityInviteCallback)(C.simple_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}
