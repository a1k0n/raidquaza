package raid

import (
	"github.com/bwmarrin/discordgo"
	"fmt"
	"strings"
	"log"
	"strconv"
	"raidquaza/gymdb"
)

func (bs *BotState) scanCommand(s *discordgo.Session, m *discordgo.MessageCreate, query string) {
	q := strings.Split(query, ",")
	if len(q) < 2 {
		s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> use !scan latitude,longitude")
		return
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(q[0]), 64)
	lon, err := strconv.ParseFloat(strings.TrimSpace(q[1]), 64)

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

