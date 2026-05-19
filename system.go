package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include "discord.h"

void isInstalled_c(bool installed, void* userData);
void freeUserData_c(void* userData);
*/
import "C"
import "unsafe"

//export goIsInstalledCallback
func goIsInstalledCallback(installed C.bool, id uintptr) {
	cb := getCallback(id).(func(bool))
	cb(bool(installed))
}

func (c *Client) RegisterLaunchCommand(applicationID uint64, command string) bool {
	cCommand := toDiscordString(command)
	defer freeDiscordString(cCommand)
	return bool(C.Discord_Client_RegisterLaunchCommand(&c.cclient, C.uint64_t(applicationID), cCommand))
}

func (c *Client) RegisterLaunchSteamApplication(applicationID uint64, steamAppID uint32) bool {
	return bool(C.Discord_Client_RegisterLaunchSteamApplication(&c.cclient, C.uint64_t(applicationID), C.uint32_t(steamAppID)))
}

func (c *Client) SetGameWindowPid(pid int) {
	C.Discord_Client_SetGameWindowPid(&c.cclient, C.int32_t(pid))
}

func (c *Client) IsDiscordAppInstalled(callback func(bool)) {
	id := registerCallback(callback)
	C.Discord_Client_IsDiscordAppInstalled(
		&c.cclient,
		(C.Discord_Client_IsDiscordAppInstalledCallback)(C.isInstalled_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}
