package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo !discord_partner_sdk LDFLAGS: -L. -ldiscord_partner_sdk
#cgo discord_partner_sdk LDFLAGS: -L${SRCDIR}/lib -ldiscord_partner_sdk
*/
import "C"
