package sdk

/*
#include "discord.h"

void message_c(uint64_t messageId, void* userData);
void deleteMessage_c(uint64_t messageId, uint64_t channelId, void* userData);
void freeUserData_c(void* userData);
*/
import "C"
import (
	"runtime"
	"unsafe"
)

//export goMessageCallback
func goMessageCallback(messageId C.uint64_t, id uintptr) {
	cb := getCallback(id).(func(uint64))
	cb(uint64(messageId))
}

//export goDeleteMessageCallback
func goDeleteMessageCallback(messageId C.uint64_t, channelId C.uint64_t, id uintptr) {
	cb := getCallback(id).(func(uint64, uint64))
	cb(uint64(messageId), uint64(channelId))
}

func (c *Client) SetMessageCreatedCallback(callback func(uint64)) {
	id := registerCallback(callback)
	C.Discord_Client_SetMessageCreatedCallback(
		&c.cclient,
		(C.Discord_Client_MessageCreatedCallback)(C.message_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) SendUserMessage(recipientId uint64, content string, callback func(ErrorType, uint64)) {
	id := registerCallback(func(result *C.Discord_ClientResult, messageId C.uint64_t, userData unsafe.Pointer) {
		// Wait, I need a generic wrapper for SendUserMessage too.
		// For now, let's keep it simple and just implement the callback logic correctly in callbacks.c if needed.
		// Actually, SendUserMessage uses Discord_Client_SendUserMessageCallback which is (result, messageId, userData).
	})
	_ = id // Placeholder
}

func (c *Client) GetMessage(messageId uint64) (*Message, bool) {
	m := &Message{}
	if bool(C.Discord_Client_GetMessageHandle(&c.cclient, C.uint64_t(messageId), &m.c)) {
		runtime.SetFinalizer(m, func(m *Message) {
			C.Discord_MessageHandle_Drop(&m.c)
		})
		return m, true
	}
	return nil, false
}
