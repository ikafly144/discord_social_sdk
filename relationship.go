package sdk

/*
#include "discord.h"
*/
import "C"
import (
	"runtime"
	"unsafe"
)

func (c *Client) GetRelationships() []*Relationship {
	var span C.struct_Discord_RelationshipHandleSpan
	C.Discord_Client_GetRelationships(&c.cclient, &span)

	if span.size == 0 {
		return nil
	}

	handles := unsafe.Slice(span.ptr, span.size)
	relationships := make([]*Relationship, span.size)
	for i := range handles {
		r := &Relationship{}
		C.Discord_RelationshipHandle_Clone(&r.c, &handles[i])
		runtime.SetFinalizer(r, func(r *Relationship) {
			C.Discord_RelationshipHandle_Drop(&r.c)
		})
		relationships[i] = r
	}
	return relationships
}
