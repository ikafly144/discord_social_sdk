package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include "discord.h"

void updateRichPresence_c(Discord_ClientResult* result, void* userData);
void freeUserData_c(void* userData);
*/
import "C"
import "unsafe"

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
