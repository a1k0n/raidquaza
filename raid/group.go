package raid

import (
	"time"
	"fmt"
	"sort"
	"strings"
	"github.com/bwmarrin/discordgo"
	"log"
)

type Group struct {
	raid      *Raid
	number    int
	StartTime time.Time      `json:"start_time"`
	Members   map[string]int `json:"members"` // discord userid set
	Expired   bool           `json:"expired"`
}

func (rg *Group) String() string {
	return fmt.Sprintf("%s%s raid at %s starting %s, ends %s", rg.raid.Emoji, rg.raid.What,
		rg.raid.Gym.Name, rg.StartTime.Format("3:04 PM"),
		rg.raid.EndTime.Format("3:04 PM"))
}

func (rg *Group) Total() int {
	total := 0
	for _, n := range rg.Members {
		total += n
	}
	return total
}

func (rg *Group) genMessage(n int) string {
	startTime := rg.StartTime.Format("3:04 PM")
	strikeThru := ""
	if rg.Expired {
		strikeThru = "~~"
	}
	return fmt.Sprintf("%d%s %s**%s** | %d attending: %s%s",
		n, boxEmoji, strikeThru, startTime, rg.Total(), rg.Mentions(), strikeThru)
}

func (rg *Group) Mentions() string {
	var mentions []string
	for mem, n := range rg.Members {
		if n > 1 {
			mentions = append(mentions, fmt.Sprintf("<@%s> (x%d)", mem, n))
		} else {
			mentions = append(mentions, "<@"+mem+">")
		}
	}
	sort.Strings(mentions)
	return strings.Join(mentions, " ")
}

func (rg *Group) Expire(s *discordgo.Session) {
	if rg.Expired {
		return
	}
	rg.Expired = true
	log.Printf("%s expired.", rg.String())
	if len(rg.Members) > 0 {
		s.ChannelMessageSend(rg.raid.ChannelID, fmt.Sprintf("%s %s raid at %s starting now!",
			rg.Mentions(), rg.StartTime.Format("3:04PM"), rg.raid.Gym.Name))
		emoji := fmt.Sprintf("%d%s", rg.number, boxEmoji)
		s.MessageReactionRemove(rg.raid.ChannelID, rg.raid.MessageID, emoji, s.State.User.ID)
		for userId := range rg.Members {
			s.MessageReactionRemove(rg.raid.ChannelID, rg.raid.MessageID, emoji, userId)
		}
	}
}

func (rg *Group) Cancel(s *discordgo.Session) {
	log.Printf("%s deleted.", rg.String())
	if len(rg.Members) > 0 {
		s.ChannelMessageSend(rg.raid.ChannelID, fmt.Sprintf("%s %s was cancelled",
			rg.Mentions(), rg.String()))
	}
}
