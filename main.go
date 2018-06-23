package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"strings"
	"raidquaza/raid"
)

const snapshotPath = "rqdata.json"

func loadAuthToken() string {
	f, err := os.Open("authtoken.txt")
	if err != nil {
		log.Fatal("authtoken.txt", err)
	}
	defer f.Close()
	buf := make([]byte, 256)
	n, err := f.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(string(buf[:n]))
}

func main() {
	dg, err := discordgo.New("Bot " + loadAuthToken())

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
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	log.Println("Shutting down.")
	botState.Stop()

	// Cleanly close down the Discord session.
	dg.Close()
}

