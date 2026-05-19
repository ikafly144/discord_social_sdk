#include "discord.h"
#include <stdint.h>
#include <stdbool.h>

// These are exported from Go
extern void goUpdateRichPresenceCallback(Discord_ClientResult* result, uintptr_t id);
extern void goLogCallback(Discord_String message, Discord_LoggingSeverity severity, uintptr_t id);
extern void goFreeUserData(uintptr_t id);
extern void goCreateOrJoinLobbyCallback(Discord_ClientResult* result, uint64_t lobbyId, uintptr_t id);
extern void goLobbyCallback(uintptr_t id);
extern void goVoiceParticipantChangedCallback(uint64_t lobbyId, uint64_t memberId, bool added, uintptr_t id);
extern void goStatusChangedCallback(int status, int error, int32_t errorDetail, uintptr_t id);
extern void goMessageCallback(uint64_t messageId, uintptr_t id);
extern void goDeleteMessageCallback(uint64_t messageId, uint64_t channelId, uintptr_t id);

extern void goAuthorizationCallback(Discord_ClientResult* result, Discord_String code, Discord_String redirectUri, uintptr_t id);
extern void goTokenExchangeCallback(Discord_ClientResult* result, Discord_String accessToken, Discord_String refreshToken, Discord_AuthorizationTokenType tokenType, int32_t expiresIn, Discord_String scopes, uintptr_t id);
extern void goSimpleCallback(Discord_ClientResult* result, uintptr_t id);

extern void goActivityInviteCallback(Discord_ActivityInvite* invite, uintptr_t id);
extern void goActivityJoinCallback(Discord_String joinSecret, uintptr_t id);
extern void goAcceptActivityInviteCallback(Discord_ClientResult* result, Discord_String joinSecret, uintptr_t id);

void updateRichPresence_c(Discord_ClientResult* result, void* userData) {
    goUpdateRichPresenceCallback(result, (uintptr_t)userData);
}

void logCallback_c(Discord_String message, Discord_LoggingSeverity severity, void* userData) {
    goLogCallback(message, severity, (uintptr_t)userData);
}

void freeUserData_c(void* userData) {
    goFreeUserData((uintptr_t)userData);
}

void createOrJoinLobby_c(Discord_ClientResult* result, uint64_t lobbyId, void* userData) {
    goCreateOrJoinLobbyCallback(result, lobbyId, (uintptr_t)userData);
}

void lobby_c(uint64_t lobbyId, void* userData) {
    goLobbyCallback((uintptr_t)userData);
}

void voiceParticipantChanged_c(uint64_t lobbyId, uint64_t memberId, bool added, void* userData) {
    goVoiceParticipantChangedCallback(lobbyId, memberId, added, (uintptr_t)userData);
}

void statusChanged_c(int status, int error, int32_t errorDetail, void* userData) {
    goStatusChangedCallback(status, error, errorDetail, (uintptr_t)userData);
}

void message_c(uint64_t messageId, void* userData) {
    goMessageCallback(messageId, (uintptr_t)userData);
}

void deleteMessage_c(uint64_t messageId, uint64_t channelId, void* userData) {
    goDeleteMessageCallback(messageId, channelId, (uintptr_t)userData);
}

void authorization_c(Discord_ClientResult* result, Discord_String code, Discord_String redirectUri, void* userData) {
    goAuthorizationCallback(result, code, redirectUri, (uintptr_t)userData);
}

void tokenExchange_c(Discord_ClientResult* result, Discord_String accessToken, Discord_String refreshToken, Discord_AuthorizationTokenType tokenType, int32_t expiresIn, Discord_String scopes, void* userData) {
    goTokenExchangeCallback(result, accessToken, refreshToken, tokenType, expiresIn, scopes, (uintptr_t)userData);
}

void simple_c(Discord_ClientResult* result, void* userData) {
    goSimpleCallback(result, (uintptr_t)userData);
}

void activityInvite_c(Discord_ActivityInvite* invite, void* userData) {
    goActivityInviteCallback(invite, (uintptr_t)userData);
}

void activityJoin_c(Discord_String joinSecret, void* userData) {
    goActivityJoinCallback(joinSecret, (uintptr_t)userData);
}

void acceptActivityInvite_c(Discord_ClientResult* result, Discord_String joinSecret, void* userData) {
    goAcceptActivityInviteCallback(result, joinSecret, (uintptr_t)userData);
}
