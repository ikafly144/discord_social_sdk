package sdk

/*
#include "discord.h"

extern void goUpdateRichPresenceCallback(Discord_ClientResult* result, uintptr_t id);
extern void goLogCallback(Discord_String message, Discord_LoggingSeverity severity, uintptr_t id);
extern void goFreeUserData(uintptr_t id);

void updateRichPresence_c(Discord_ClientResult* result, void* userData);
void logCallback_c(Discord_String message, Discord_LoggingSeverity severity, void* userData);
void freeUserData_c(void* userData);
*/
import "C"
import (
	"runtime"
	"unsafe"
)

//export goUpdateRichPresenceCallback
func goUpdateRichPresenceCallback(result *C.Discord_ClientResult, id uintptr) {
	cb := getCallback(id).(func(ErrorType))
	errType := C.Discord_ClientResult_Type(result)
	cb(ErrorType(errType))
}

//export goLogCallback
func goLogCallback(message C.Discord_String, severity C.Discord_LoggingSeverity, id uintptr) {
	cb := getCallback(id).(func(string, LoggingSeverity))
	cb(fromDiscordString(message), LoggingSeverity(severity))
}

//export goFreeUserData
func goFreeUserData(id uintptr) {
	unregisterCallback(id)
}

func NewClient() *Client {
	c := &Client{}
	c.init()
	runtime.SetFinalizer(c, func(c *Client) {
		c.close()
	})
	return c
}

func NewClientWithOptions(options *ClientCreateOptions) *Client {
	c := &Client{}
	C.Discord_Client_InitWithOptions(&c.cclient, &options.c)
	runtime.SetFinalizer(c, func(c *Client) {
		c.close()
	})
	return c
}

type Client struct {
	cclient C.struct_Discord_Client
}

func (c *Client) init() {
	C.Discord_Client_Init(&c.cclient)
}

func (c *Client) close() {
	C.Discord_Client_Drop(&c.cclient)
}

func (c *Client) Connect() {
	C.Discord_Client_Connect(&c.cclient)
}

func (c *Client) Disconnect() {
	C.Discord_Client_Disconnect(&c.cclient)
}

func (c *Client) SetApplicationID(id uint64) {
	C.Discord_Client_SetApplicationId(&c.cclient, C.uint64_t(id))
}

func (c *Client) RunCallbacks() {
	C.Discord_RunCallbacks()
}

func (c *Client) GetCurrentUser() *User {
	u := &User{}
	C.Discord_Client_GetCurrentUser(&c.cclient, &u.c)
	runtime.SetFinalizer(u, func(u *User) {
		C.Discord_UserHandle_Drop(&u.c)
	})
	return u
}

func (c *Client) AddLogCallback(minSeverity LoggingSeverity, callback func(string, LoggingSeverity)) {
	id := registerCallback(callback)
	C.Discord_Client_AddLogCallback(
		&c.cclient,
		(C.Discord_Client_LogCallback)(C.logCallback_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
		C.Discord_LoggingSeverity(minSeverity),
	)
}
