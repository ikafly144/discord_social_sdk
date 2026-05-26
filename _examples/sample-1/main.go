package main

import (
	"flag"
	"fmt"
	"strconv"
	"time"

	sdk "github.com/ikafly144/discord_social_sdk"
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

	client := sdk.NewClient()

	client.AddLogCallback(sdk.LoggingSeverityInfo, func(msg string, severity sdk.LoggingSeverity) {
		fmt.Printf("[Discord] %v: %s\n", severity, msg)
	})

	client.SetApplicationID(appID)

	fmt.Println("Connecting...")
	client.Connect()

	// Need to run callbacks for the connection and other events to process
	go func() {
		for {
			client.RunCallbacks()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	fmt.Println("Waiting for UserHandle info...")
	time.Sleep(2 * time.Second)

	UserHandle := client.GetCurrentUser()
	fmt.Printf("Logged in as: %s#%s (ID: %d)\n", UserHandle.Username(), "0000", UserHandle.ID())

	relationships := client.GetRelationships()
	fmt.Printf("Relationships (%d):\n", len(relationships))
	for _, r := range relationships {
		u := r.UserHandle()
		if u != nil {
			fmt.Printf("- %s (ID: %d) Type: %v\n", u.Username(), u.ID(), r.Type())
		}
	}

	client.RegisterLaunchCommand(appID, "awesome-go-wrapper://launch")

	activity := sdk.NewActivity()
	activity.SetType(sdk.ActivityTypesPlaying)
	activity.SetName("Go Wrapper Test")
	activity.SetState("Working on Go Wrapper")
	activity.SetDetails("Implementing Discord Social SDK")

	party := sdk.NewActivityParty()
	party.SetID("test123")
	party.SetCurrentSize(1)
	party.SetMaxSize(5)
	activity.SetParty(party)

	secrets := sdk.NewActivitySecrets()
	secrets.SetJoin("joinSecret")
	activity.SetSecrets(secrets)

	client.UpdateRichPresence(activity, func(err sdk.ErrorType) {
		if err == sdk.ErrorTypeNone {
			fmt.Println("Rich presence updated!")
		} else {
			fmt.Printf("Failed to update rich presence: %v\n", err)
		}
	})

	fmt.Println("Press Enter to exit")
	fmt.Scanln()
}


