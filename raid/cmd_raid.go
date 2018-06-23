package raid

import (
	"github.com/bwmarrin/discordgo"
	"time"
	"fmt"
	"strings"
	"log"
)

func (bs *BotState) raidCommand(s *discordgo.Session, m *discordgo.MessageCreate, query string) {
	// !raid ttar foo bar place ends at 4:00
	// !raid thing foo bar place ends in 23:51
	// !raid egg foo bar place ends in 15
	r := &Raid{
		RequestMsgID: m.ID,
		ChannelID: m.ChannelID,
	}
	err, gymmatches := r.ParseRaidRequest(query, bs.gymdb, time.Now())
	if err == ErrNoEnd {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> You need to tell me an end time. Use `!raid <pokemon> @ <location> [hatches/ends] [at/in] <time>`",
			m.Author.ID))
		return
	} else if err == ErrNoMatches {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> Couldn't find the gym you're looking for", m.Author.ID))
		return
	} else if err == ErrNonUnique {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> Which gym did you mean?\n%s",
			m.Author.ID, strings.Join(formatGymMatches(gymmatches, nil), "\n")))
		return
	} else if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> Didn't understand. Use `!raid <pokemon> @ <location> ends [at/in] <time>`",
			m.Author.ID))
		log.Print("error parsing raid request", err)
		return
	}

	messageData := discordgo.MessageSend{
		Content: r.GenMessage(),
	}
	addGymEmbed(r.Gym, &messageData)

	msgId, err := s.ChannelMessageSendComplex(m.ChannelID, &messageData)
	if err != nil {
		log.Print(err)
		return
	}
	r.MessageID = msgId.ID

	s.ChannelMessagePin(m.ChannelID, msgId.ID)

	err = s.MessageReactionAdd(m.ChannelID, msgId.ID, "⏰")
	if err != nil {
		s.ChannelMessageDelete(m.ChannelID, msgId.ID)
		log.Print(err)
		return
	}

	log.Printf("added [%s] %s", msgId.ID, r.String())
	// acknowledge the original message with an emoji
	rqemoji, ok := bs.guildEmoji("Raidquaza")
	if ok {
		log.Printf("ack with rq emoji: %s", rqemoji)
		s.MessageReactionAdd(m.ChannelID, m.ID, rqemoji)
	} else {
		log.Print("no raidquaza emoji found")
	}

	if len(r.Groups) > 0 {
		s.MessageReactionAdd(m.ChannelID, msgId.ID, "➕")
		s.MessageReactionAdd(m.ChannelID, msgId.ID, "➖")

		for n := range r.Groups {
			s.MessageReactionAdd(m.ChannelID, msgId.ID,
				fmt.Sprintf("%d%s", n+1, boxEmoji))
		}
	}

	bs.mut.Lock()
	bs.activeMessages[m.ID] = &Request{r}
	bs.Raids[msgId.ID] = r
	bs.dirty = true
	bs.mut.Unlock()
}

