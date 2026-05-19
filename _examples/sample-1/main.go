package main

import (
	"flag"
	"fmt"
	"strconv"
	"time"

	sdk "git.sabafly.net/ikafly144/discord_social_sdk"
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

	fmt.Println("Waiting for user info...")
	time.Sleep(2 * time.Second)

	user := client.GetCurrentUser()
	fmt.Printf("Logged in as: %s#%s (ID: %d)\n", user.Username(), "0000", user.ID())

	relationships := client.GetRelationships()
	fmt.Printf("Relationships (%d):\n", len(relationships))
	for _, r := range relationships {
		u := r.User()
		if u != nil {
			fmt.Printf("- %s (ID: %d) Type: %v\n", u.Username(), u.ID(), r.Type())
		}
	}

	activity := sdk.NewActivity()
	activity.SetType(sdk.ActivityTypePlaying)
	activity.SetName("Go Wrapper Test")
	activity.SetState("Working on Go Wrapper")
	activity.SetDetails("Implementing Discord Social SDK")

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
