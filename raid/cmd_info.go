package raid

import (
	"github.com/bwmarrin/discordgo"
	"fmt"
	"strings"
	"log"
	"raidquaza/gymdb"
)

func formatGymMatches(gs []*gymdb.Gym, scores []float32) []string {
	var matches []string
	for i, g := range gs {
		if scores == nil {
			matches = append(matches, fmt.Sprintf(
				"  [gym `%s`] %s %s <https://www.google.com/maps/?q=%f,%f>",
				g.Id, g.Name, g.StreetAddr, g.Latitude, g.Longitude))
		} else {
			matches = append(matches, fmt.Sprintf(
				"  %0.1f%% [gym `%s`] %s %s <https://www.google.com/maps/?q=%f,%f>",
				scores[i]*100.0, g.Id, g.Name, g.StreetAddr, g.Latitude, g.Longitude))
		}
	}
	return matches
}

func (bs *BotState) infoCommand(s *discordgo.Session, m *discordgo.MessageCreate, query string) {
	gs, scores := bs.gymdb.GetGyms(query, 0.5)
	if len(gs) == 0 {
		s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> couldn't find a matching gym")
		return
	}

	messageData := discordgo.MessageSend{}

	if len(gs) == 1 {
		g := gs[0]
		messageData.Content = fmt.Sprintf("<@%s> [gym `%s`] %s | %s",
			m.Author.ID, g.Id, g.Name, g.StreetAddr)
		addGymEmbed(g, &messageData)
	} else {
		matches := []string{fmt.Sprintf("<@%s> `%s` could be:", m.Author.ID, query)}
		matches = append(matches, formatGymMatches(gs, scores)...)
		messageData.Content = strings.Join(matches, "\n")
	}

	_, err := s.ChannelMessageSendComplex(m.ChannelID, &messageData)
	if err != nil {
		log.Print(err)
	}
	return
}

