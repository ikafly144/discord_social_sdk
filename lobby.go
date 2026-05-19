package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include "discord.h"

void createOrJoinLobby_c(Discord_ClientResult* result, uint64_t lobbyId, void* userData);
void lobby_c(uint64_t lobbyId, void* userData);
void lobbyMember_c(uint64_t lobbyId, uint64_t memberId, void* userData);
void simple_c(Discord_ClientResult* result, void* userData);
void freeUserData_c(void* userData);
*/
import "C"
import (
	"runtime"
	"unsafe"
)

//export goCreateOrJoinLobbyCallback
func goCreateOrJoinLobbyCallback(result *C.Discord_ClientResult, lobbyId C.uint64_t, id uintptr) {
	cb := getCallback(id).(func(ErrorType, uint64))
	errType := C.Discord_ClientResult_Type(result)
	cb(ErrorType(errType), uint64(lobbyId))
}

//export goLobbyCallback
func goLobbyCallback(lobbyId C.uint64_t, id uintptr) {
	cb := getCallback(id).(func(uint64))
	cb(uint64(lobbyId))
}

//export goLobbyMemberCallback
func goLobbyMemberCallback(lobbyId C.uint64_t, memberId C.uint64_t, id uintptr) {
	cb := getCallback(id).(func(uint64, uint64))
	cb(uint64(lobbyId), uint64(memberId))
}

func (c *Client) CreateOrJoinLobby(secret string, callback func(ErrorType, uint64)) {
	id := registerCallback(callback)
	s := toDiscordString(secret)
	defer freeDiscordString(s)
	C.Discord_Client_CreateOrJoinLobby(
		&c.cclient,
		s,
		(C.Discord_Client_CreateOrJoinLobbyCallback)(C.createOrJoinLobby_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SetLobbyCreatedCallback(callback func(uint64)) {
	id := registerCallback(callback)
	C.Discord_Client_SetLobbyCreatedCallback(
		&c.cclient,
		(C.Discord_Client_LobbyCreatedCallback)(C.lobby_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SetLobbyDeletedCallback(callback func(uint64)) {
	id := registerCallback(callback)
	C.Discord_Client_SetLobbyDeletedCallback(
		&c.cclient,
		(C.Discord_Client_LobbyDeletedCallback)(C.lobby_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SetLobbyMemberAddedCallback(callback func(uint64, uint64)) {
	id := registerCallback(callback)
	C.Discord_Client_SetLobbyMemberAddedCallback(
		&c.cclient,
		(C.Discord_Client_LobbyMemberAddedCallback)(C.lobbyMember_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SetLobbyMemberRemovedCallback(callback func(uint64, uint64)) {
	id := registerCallback(callback)
	C.Discord_Client_SetLobbyMemberRemovedCallback(
		&c.cclient,
		(C.Discord_Client_LobbyMemberRemovedCallback)(C.lobbyMember_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SetLobbyMemberUpdatedCallback(callback func(uint64, uint64)) {
	id := registerCallback(callback)
	C.Discord_Client_SetLobbyMemberUpdatedCallback(
		&c.cclient,
		(C.Discord_Client_LobbyMemberUpdatedCallback)(C.lobbyMember_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SetLobbyUpdatedCallback(callback func(uint64)) {
	id := registerCallback(callback)
	C.Discord_Client_SetLobbyUpdatedCallback(
		&c.cclient,
		(C.Discord_Client_LobbyUpdatedCallback)(C.lobby_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) LinkChannelToLobby(lobbyID uint64, channelID uint64, callback func(ErrorType)) {
	id := registerCallback(callback)
	C.Discord_Client_LinkChannelToLobby(
		&c.cclient,
		C.uint64_t(lobbyID),
		C.uint64_t(channelID),
		(C.Discord_Client_LinkOrUnlinkChannelCallback)(C.simple_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) UnlinkChannelFromLobby(lobbyID uint64, callback func(ErrorType)) {
	id := registerCallback(callback)
	C.Discord_Client_UnlinkChannelFromLobby(
		&c.cclient,
		C.uint64_t(lobbyID),
		(C.Discord_Client_LinkOrUnlinkChannelCallback)(C.simple_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) GetLobby(id uint64) (*Lobby, bool) {
	l := &Lobby{}
	if bool(C.Discord_Client_GetLobbyHandle(&c.cclient, C.uint64_t(id), &l.c)) {
		runtime.SetFinalizer(l, func(l *Lobby) {
			C.Discord_LobbyHandle_Drop(&l.c)
		})
		return l, true
	}
	return nil, false
}
