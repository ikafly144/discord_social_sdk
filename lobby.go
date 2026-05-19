package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include "discord.h"

void createOrJoinLobby_c(Discord_ClientResult* result, uint64_t lobbyId, void* userData);
void lobby_c(uint64_t lobbyId, void* userData);
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
func goLobbyCallback(id uintptr) {
	cb := getCallback(id).(func())
	cb()
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

func (c *Client) SetLobbyCreatedCallback(callback func()) {
	id := registerCallback(callback)
	C.Discord_Client_SetLobbyCreatedCallback(
		&c.cclient,
		(C.Discord_Client_LobbyCreatedCallback)(C.lobby_c),
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
