package raid

import (
	"raidquaza/gymdb"
	"time"
	"fmt"
	"strings"
	"github.com/bwmarrin/discordgo"
	"log"
)

type Raid struct {
	Gym       *gymdb.Gym `json:"gym"`
	What      string     `json:"what"`
	Emoji     string     `json:"emoji"` // latest reaction emoji, can indicate which pokemon
	EndTime   time.Time  `json:"end_time"`
	MessageID string     `json:"msg_id"`     // discord pinned message id
	ChannelID string     `json:"channel_id"` // discord channel pinned in
	Groups    []*Group   `json:"groups"`
	expired   bool
}

var boxEmoji = string([]byte{226, 131, 163})

func (r *Raid) GenMessage() string {
	clockMsg := ""
	if !r.expired {
		if len(r.Groups) == 0 {
			clockMsg = "\nClick ⏰ to add a raid group time."
		} else {
			clockMsg = "\nClick 🔢 to join group, ⏰ to add new time."
		}
	}
	mapUrl := fmt.Sprintf("https://www.google.com/maps/?q=%f,%f",
		r.Gym.Latitude, r.Gym.Longitude)
	var groupMsgs []string
	for n, rg := range r.Groups {
		groupMsgs = append(groupMsgs, rg.genMessage(n+1))
	}
	return fmt.Sprintf("**%s%s** expires %s\n%s | %s %s%s\n%s",
		r.Emoji, r.What, r.EndTime.Format("3:04 PM"), r.Gym.Name,
		r.Gym.StreetAddr, mapUrl, clockMsg, strings.Join(groupMsgs, "\n"))
}

func (r *Raid) String() string {
	return fmt.Sprintf("%s%s raid at %s until %s", r.Emoji, r.What, r.Gym.Name, r.EndTime.Format("3:04 PM"))
}

func (r *Raid) SendUpdate(s *discordgo.Session) {
	s.ChannelMessageEdit(r.ChannelID, r.MessageID, r.GenMessage())
}

func (r *Raid) AddGroup(startTime time.Time, s *discordgo.Session) *Group {
	n := len(r.Groups) + 1
	rg := &Group{
		raid:      r,
		number:    n,
		StartTime: startTime,
		Members:   make(map[string]int),
	}
	r.Groups = append(r.Groups, rg)
	s.MessageReactionAdd(r.ChannelID, r.MessageID, fmt.Sprintf("%d%s", n, boxEmoji))
	s.MessageReactionAdd(r.ChannelID, r.MessageID, "➕")
	s.MessageReactionAdd(r.ChannelID, r.MessageID, "➖")
	r.SendUpdate(s)

	return rg
}

func (r *Raid) Expire(s *discordgo.Session) {
	if r.expired {
		return
	}
	r.expired = true
	log.Printf("%s expired.", r.String())
	s.ChannelMessageUnpin(r.ChannelID, r.MessageID)
	r.SendUpdate(s)
	s.MessageReactionsRemoveAll(r.ChannelID, r.MessageID)
}

func (r *Raid) UpdateGroupPointers() {
	for _, rg := range r.Groups {
		rg.raid = r
	}
}
