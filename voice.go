package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include <stdbool.h>
#include "discord.h"

void voiceParticipantChanged_c(uint64_t lobbyId, uint64_t memberId, bool added, void* userData);
void statusChanged_c(int status, int error, int32_t errorDetail, void* userData);
void freeUserData_c(void* userData);
*/
import "C"
import (
	"runtime"
	"unsafe"
)

//export goVoiceParticipantChangedCallback
func goVoiceParticipantChangedCallback(lobbyId C.uint64_t, memberId C.uint64_t, added C.bool, id uintptr) {
	cb := getCallback(id).(func(uint64, uint64, bool))
	cb(uint64(lobbyId), uint64(memberId), bool(added))
}

//export goStatusChangedCallback
func goStatusChangedCallback(status C.int, error C.int, errorDetail C.int32_t, id uintptr) {
	cb := getCallback(id).(func(int, int, int32))
	cb(int(status), int(error), int32(errorDetail))
}

func (c *Client) SetVoiceParticipantChangedCallback(callback func(uint64, uint64, bool)) {
	id := registerCallback(callback)
	C.Discord_Client_SetVoiceParticipantChangedCallback(
		&c.cclient,
		(C.Discord_Client_VoiceParticipantChangedCallback)(C.voiceParticipantChanged_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) StartCall(channelId uint64) (*Call, bool) {
	call := &Call{}
	if bool(C.Discord_Client_StartCall(&c.cclient, C.uint64_t(channelId), &call.c)) {
		runtime.SetFinalizer(call, func(c *Call) {
			C.Discord_Call_Drop(&c.c)
		})
		return call, true
	}
	return nil, false
}

func (c *Client) SetSelfMuteAll(mute bool) {
	C.Discord_Client_SetSelfMuteAll(&c.cclient, C.bool(mute))
}
