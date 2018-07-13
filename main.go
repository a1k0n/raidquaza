package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"raidquaza/raid"
	"raidquaza/util"
)

const snapshotPath = "rqdata.json"

func main() {
	dg, err := discordgo.New("Bot " + util.LoadAuthToken("authtoken.txt"))

	if err != nil {
		log.Fatal(err)
	}

	botState := raid.NewBotState(dg, snapshotPath, "gymdb/gyms.txt")

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down.")
	botState.Stop()

	// Cleanly close down the Discord session.
	dg.Close()
}

