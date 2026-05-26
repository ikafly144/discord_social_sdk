package main

import (
	"flag"
	"fmt"
	"runtime"
	"strconv"
	"time"

	discord "github.com/ikafly144/discord_social_sdk"
)

func main() {
	appIDFlag := flag.String("appid", "", "Application ID")
	flag.Parse()

	if *appIDFlag == "" {
		fmt.Println("Please provide an Application ID")
		return
	}
	appID, err := strconv.ParseUint(*appIDFlag, 10, 64)
	if err != nil {
		fmt.Printf("Invalid Application ID: %v\n", err)
		return
	}

	client := discord.NewClient()

	client.AddLogCallback(func(arg0 string, arg1 discord.Discord_LoggingSeverity) {
		fmt.Printf("[Discord SDK] %s\n", arg0)
	}, discord.Discord_LoggingSeverity_Info)

	client.SetApplicationId(appID)

	// Need to run callbacks for the connection and other events to process
	go func() {
		for {
			runtime.LockOSThread()
			discord.RunCallbacks()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	client.SetStatusChangedCallback(func(status discord.Discord_Client_Status, err discord.Discord_Client_Error, errorDetails int32) {
		if err != discord.Discord_Client_Error_None {
			fmt.Printf("Status changed with error: %v (details: %d)\n", err, errorDetails)
			return
		}
		if status != discord.Discord_Client_Status_Ready {
			fmt.Printf("Status changed: %v\n", status)
			return
		}
		fmt.Println("Client is ready!")
		UserHandle := client.GetCurrentUser()
		fmt.Printf("Logged in as: %s#%s (ID: %d)\n", UserHandle.Username(), "0000", UserHandle.Id())

		relationships := client.GetRelationships()
		fmt.Printf("Relationships (%d):\n", len(relationships))
		for _, r := range relationships {
			u, ok := r.User()
			if ok {
				fmt.Printf("- %s (ID: %d) Type: %v\n", u.Username(), u.Id(), r.DiscordRelationshipType())
			}
		}
	})

	codeVerifier := client.CreateAuthorizationCodeVerifier()
	authArgs := discord.NewAuthorizationArgs()
	authArgs.SetClientId(appID)
	authArgs.SetScopes(discord.Client_GetDefaultCommunicationScopes())
	authArgs.SetCodeChallenge(new(codeVerifier.Challenge()))
	client.Authorize(authArgs, func(arg0 *discord.Discord_ClientResult, arg1, arg2 string) {
		if !arg0.Successful() {
			fmt.Printf("Authorization failed: %v\n", arg0.Error())
			return
		}
		fmt.Printf("Authorization successful! Code: %s, Redirect URI: %s\n", arg1, arg2)

		client.GetToken(appID, arg1, codeVerifier.Verifier(), arg2,
			func(arg0 *discord.Discord_ClientResult, accessToken, refreshToken string, tokenType discord.Discord_AuthorizationTokenType, expiresIn int32, scope string) {
				if !arg0.Successful() {
					fmt.Printf("Token exchange failed: %v\n", arg0.Error())
					return
				}
				client.UpdateToken(tokenType, accessToken, func(arg0 *discord.Discord_ClientResult) {
					if !arg0.Successful() {
						fmt.Printf("Failed to update token: %v\n", arg0.Error())
						return
					}
					fmt.Println("Token updated successfully")
					client.Connect()
				})
			})
	})

	client.RegisterLaunchCommand(appID, "awesome-go-wrapper://launch")

	activity := discord.NewActivity()
	activity.SetType(discord.Discord_ActivityTypes_Playing)
	activity.SetName("Go Wrapper Test")
	activity.SetState(new("Working on Go Wrapper"))
	activity.SetDetails(new("Implementing Discord Social SDK"))

	party := discord.NewActivityParty()
	party.SetId("test123")
	party.SetCurrentSize(1)
	party.SetMaxSize(5)
	party.SetPrivacy(discord.Discord_ActivityPartyPrivacy_Private)
	activity.SetParty(party)

	secrets := discord.NewActivitySecrets()
	secrets.SetJoin("joinSecret")
	activity.SetSecrets(secrets)

	client.UpdateRichPresence(activity, func(arg0 *discord.Discord_ClientResult) {
		if arg0.Successful() {
			fmt.Println("Rich presence updated successfully")
		} else {
			fmt.Printf("Failed to update rich presence: %v\n", arg0.Error())
		}
	})

	fmt.Println("Press Enter to exit")
	fmt.Scanln()
}
