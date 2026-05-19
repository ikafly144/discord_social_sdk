package sdk

import (
	"sync"
)

var (
	callbackRegistry = make(map[uintptr]interface{})
	callbackMutex    sync.Mutex
	nextCallbackID   uintptr
)

func registerCallback(cb interface{}) uintptr {
	callbackMutex.Lock()
	defer callbackMutex.Unlock()
	nextCallbackID++
	callbackRegistry[nextCallbackID] = cb
	return nextCallbackID
}

func getCallback(id uintptr) interface{} {
	callbackMutex.Lock()
	defer callbackMutex.Unlock()
	return callbackRegistry[id]
}

func unregisterCallback(id uintptr) {
	callbackMutex.Lock()
	defer callbackMutex.Unlock()
	delete(callbackRegistry, id)
}
