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
