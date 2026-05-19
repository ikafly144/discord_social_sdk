package sdk

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include "discord.h"

void authorization_c(Discord_ClientResult* result, Discord_String code, Discord_String redirectUri, void* userData);
void tokenExchange_c(Discord_ClientResult* result, Discord_String accessToken, Discord_String refreshToken, Discord_AuthorizationTokenType tokenType, int32_t expiresIn, Discord_String scopes, void* userData);
void simple_c(Discord_ClientResult* result, void* userData);
void freeUserData_c(void* userData);
*/
import "C"
import (
	"unsafe"
)

type TokenExchangeResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    AuthorizationTokenType
	ExpiresIn    int
	Scopes       string
}

//export goAuthorizationCallback
func goAuthorizationCallback(result *C.Discord_ClientResult, code C.Discord_String, redirectUri C.Discord_String, id uintptr) {
	cb := getCallback(id).(func(ErrorType, string, string))
	cb(ErrorType(C.Discord_ClientResult_Type(result)), fromDiscordString(code), fromDiscordString(redirectUri))
}

//export goTokenExchangeCallback
func goTokenExchangeCallback(result *C.Discord_ClientResult, accessToken C.Discord_String, refreshToken C.Discord_String, tokenType C.Discord_AuthorizationTokenType, expiresIn C.int32_t, scopes C.Discord_String, id uintptr) {
	cb := getCallback(id).(func(ErrorType, TokenExchangeResult))
	cb(ErrorType(C.Discord_ClientResult_Type(result)), TokenExchangeResult{
		AccessToken:  fromDiscordString(accessToken),
		RefreshToken: fromDiscordString(refreshToken),
		TokenType:    AuthorizationTokenType(tokenType),
		ExpiresIn:    int(expiresIn),
		Scopes:       fromDiscordString(scopes),
	})
}

//export goSimpleCallback
func goSimpleCallback(result *C.Discord_ClientResult, id uintptr) {
	cb := getCallback(id).(func(ErrorType))
	cb(ErrorType(C.Discord_ClientResult_Type(result)))
}

func (c *Client) Authorize(args *AuthorizationArgs, callback func(ErrorType, string, string)) {
	id := registerCallback(callback)
	C.Discord_Client_Authorize(
		&c.cclient,
		&args.c,
		(C.Discord_Client_AuthorizationCallback)(C.authorization_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) GetToken(applicationID uint64, code string, codeVerifier string, redirectURI string, callback func(ErrorType, TokenExchangeResult)) {
	id := registerCallback(callback)
	cCode := toDiscordString(code)
	cVerifier := toDiscordString(codeVerifier)
	cRedirect := toDiscordString(redirectURI)
	defer freeDiscordString(cCode)
	defer freeDiscordString(cVerifier)
	defer freeDiscordString(cRedirect)

	C.Discord_Client_GetToken(
		&c.cclient,
		C.uint64_t(applicationID),
		cCode,
		cVerifier,
		cRedirect,
		(C.Discord_Client_TokenExchangeCallback)(C.tokenExchange_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) RefreshToken(applicationID uint64, refreshToken string, callback func(ErrorType, TokenExchangeResult)) {
	id := registerCallback(callback)
	cToken := toDiscordString(refreshToken)
	defer freeDiscordString(cToken)

	C.Discord_Client_RefreshToken(
		&c.cclient,
		C.uint64_t(applicationID),
		cToken,
		(C.Discord_Client_TokenExchangeCallback)(C.tokenExchange_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}

func (c *Client) RevokeToken(applicationID uint64, token string, callback func(ErrorType)) {
	id := registerCallback(callback)
	cToken := toDiscordString(token)
	defer freeDiscordString(cToken)

	C.Discord_Client_RevokeToken(
		&c.cclient,
		C.uint64_t(applicationID),
		cToken,
		(C.Discord_Client_RevokeTokenCallback)(C.simple_c),
		(C.Discord_FreeFn)(C.freeUserData_c),
		unsafe.Pointer(id),
	)
}
