package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo discord_partner_sdk LDFLAGS: -ldiscord_partner_sdk
#cgo !discord_partner_sdk LDFLAGS: -L${SRCDIR}/sdk/bin/release -ldiscord_partner_sdk
*/
import "C"
