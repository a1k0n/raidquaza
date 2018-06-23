package raid

import (
	"github.com/bwmarrin/discordgo"
	"fmt"
	"strings"
	"log"
	"raidquaza/gymdb"
	"raidquaza/util"
)

func (bs *BotState) scanCommand(s *discordgo.Session, m *discordgo.MessageCreate, query string) {
	lat, lon, _, err := util.ParseLatLong(strings.Split(query, " "))
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> can't parse your lat/lon; example: -37.123,121.85")
	}

	gyms, err := gymdb.ScrapeGymhuntr(lat, lon)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> error scanning: " + err.Error())
	}
	if len(gyms) > 10 {
		gyms = gyms[:10]
	}

	matches := []string{fmt.Sprintf("<@%s> gyms around %f,%f:", m.Author.ID, lat, lon)}
	matches = append(matches, formatGymMatches(gyms, nil)...)

	_, err = s.ChannelMessageSend(m.ChannelID, strings.Join(matches, "\n"))
	if err != nil {
		log.Print(err)
	}
}

