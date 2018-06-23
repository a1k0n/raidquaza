package raid

import (
	"github.com/bwmarrin/discordgo"
	"time"
	"log"
	"strings"
)

// proxy type for the original raid request message; editing it can update the raid
type Request struct {
	Raid *Raid
}

func (r *Request) OnReactionAdd(bs *BotState, s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	// no-op
}

func (r *Request) OnReactionRemove(bs *BotState, s *discordgo.Session, m *discordgo.MessageReactionRemove) {
	// no-op
}

func (r *Request) OnMessageEdit(bs *BotState, s *discordgo.Session, m *discordgo.MessageUpdate) {
	log.Printf("editing raid %s", r.Raid.String())
	splitMsg := strings.SplitN(m.Content[len(commandLeader):], " ", 2)
	err, _ := r.Raid.ParseRaidRequest(splitMsg[1], bs.gymdb, time.Now())
	if err == nil {
		r.Raid.SendUpdate(s)
	} else {
		log.Printf("can't understand raid request: %s", err)
	}
}

func (r *Request) OnMessageDelete(bs *BotState, s *discordgo.Session, m *discordgo.MessageDelete) {
	log.Printf("Deleting raid %s", r.Raid.String())
	for _, rg := range r.Raid.Groups {
		rg.Cancel(s)
	}
	s.ChannelMessageDelete(r.Raid.ChannelID, r.Raid.MessageID)
	bs.mut.Lock()
	defer bs.mut.Unlock()
	delete(bs.Raids, m.Message.ID)
	bs.dirty = true
}
