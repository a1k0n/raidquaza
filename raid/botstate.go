package raid

import (
	"raidquaza/gymdb"
	"github.com/bwmarrin/discordgo"
	"sync"
	"os"
	"encoding/json"
	"log"
	"time"
	"fmt"
	"strings"
)

type ActiveMessage interface {
	OnReactionAdd(bs *BotState, s *discordgo.Session, m *discordgo.MessageReactionAdd)
	OnReactionRemove(bs *BotState, s *discordgo.Session, m *discordgo.MessageReactionRemove)
	OnMessageEdit(bs *BotState, s *discordgo.Session, m *discordgo.MessageUpdate)
	OnMessageDelete(bs *BotState, s *discordgo.Session, m *discordgo.MessageDelete)
}

const commandLeader = "!" // all commands to the botbegin with this character

type BotState struct {
	emojiMap     map[string]string // emoji name -> emoji id
	channelCache map[string]string // userid -> privmsg channel id
	gymdb        *gymdb.GymDB

	Raids map[string]*Raid `json:"raids"` // message id -> raid

	channelCallbacks map[string]func(*discordgo.Session, *discordgo.MessageCreate)
	activeMessages   map[string]ActiveMessage

	dirty bool

	// giant global lock
	mut          sync.Mutex
	expireTicker *time.Ticker
}

var globalEmojiMap map[string]string

func (bs *BotState) RemoveActiveMessage(messageID string) {
	bs.mut.Lock()
	defer bs.mut.Unlock()
	delete(bs.activeMessages, messageID)
}

func (bs *BotState) guildEmoji(emojiName string) (string, bool) {
	emojiId, ok := bs.emojiMap[emojiName]
	if !ok {
		return "", false
	}
	return fmt.Sprintf(":%s:%s", emojiName, emojiId), true
}

func (bs *BotState) Save(path string) error {
	tmpPath := path + "_tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	m, err := json.Marshal(bs)
	if err != nil {
		return err
	}
	f.Write(m)
	f.Close()
	os.Remove(path)
	os.Rename(tmpPath, path)
	log.Printf("Saved state snapshot to %s", path)
	return nil
}

func (bs *BotState) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	err = dec.Decode(bs)
	if err != nil {
		log.Print(err)
		return err
	}

	// fixup raid pointers not serialized
	for _, r := range bs.Raids {
		r.UpdateGroupPointers()
	}

	log.Printf("Successfully loaded shapshot %s", path)
	return nil
}

func (bs *BotState) ExpireOld(s *discordgo.Session, t time.Time) {
	bs.mut.Lock()
	defer bs.mut.Unlock()

	for k, raid := range bs.Raids {
		needUpdate := false
		for _, rg := range raid.Groups {
			if !rg.Expired && t.After(rg.StartTime) {
				rg.Expire(s)
				needUpdate = true
				bs.dirty = true
			}
		}

		if !raid.Hatched && t.After(raid.EndTime.Add(-RaidDuration)) {
			raid.Hatched = true
			needUpdate = true
		}

		if t.After(raid.EndTime) {
			raid.Expire(s)
			delete(bs.Raids, k)
			delete(bs.activeMessages, raid.RequestMsgID)
			bs.dirty = true
		} else if needUpdate {
			raid.SendUpdate(s)
		}
	}
}

func NewBotState(dg *discordgo.Session, snapshotPath string, gympath string) *BotState {
	bs := &BotState{
		emojiMap:     make(map[string]string),
		channelCache: make(map[string]string),
		gymdb:        gymdb.NewGymDB(gympath),

		Raids:            make(map[string]*Raid),
		channelCallbacks: make(map[string]func(*discordgo.Session, *discordgo.MessageCreate)),
		activeMessages:   make(map[string]ActiveMessage),
	}

	globalEmojiMap = bs.emojiMap

	bs.Load(snapshotPath)

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(bs.readyHandler)
	dg.AddHandler(bs.messageCreate)
	dg.AddHandler(bs.messageEdit)
	dg.AddHandler(bs.messageDelete)
	dg.AddHandler(bs.messageReactionAdd)
	dg.AddHandler(bs.messageReactionRemove)

	bs.expireTicker = time.NewTicker(10 * time.Second)
	go func(t *time.Ticker) {
		for range t.C {
			bs.ExpireOld(dg, time.Now())
			if bs.dirty {
				bs.Save(snapshotPath)
				bs.dirty = false
			}
		}
	}(bs.expireTicker)

	return bs
}

func (bs *BotState) Stop() {
	bs.expireTicker.Stop()
}

func (bs *BotState) readyHandler(s *discordgo.Session, r *discordgo.Ready) {
	log.Println("Ready.")
	_, err := s.UserUpdate("", "", "Raidquaza", "", "")
	if err != nil {
		log.Print(err)
	}

	guilds, err := s.UserGuilds(100, "", "")
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Member of guilds: ")
	for _, guild := range guilds {
		log.Printf("%s(%s) ", guild.Name, guild.ID)
		g, err := s.Guild(guild.ID)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, emoji := range g.Emojis {
			bs.emojiMap[emoji.Name] = emoji.ID
		}
	}
	log.Println()
}

func (bs *BotState) messageReactionRemove(s *discordgo.Session, m *discordgo.MessageReactionRemove) {
	if m.UserID == s.State.User.ID {
		return
	}

	log.Printf("messageid %s %s reaction removed: %s(%s)", m.MessageID, m.UserID, m.Emoji.ID, m.Emoji.Name)

	if m.Emoji.Name[1:] == boxEmoji {
		n := m.Emoji.Name[0] - '1'
		bs.mut.Lock()
		defer bs.mut.Unlock()
		raid, ok := bs.Raids[m.MessageID]
		if !ok {
			return
		}

		if n < 0 || n >= uint8(len(raid.Groups)) {
			return
		}
		rg := raid.Groups[n]
		if rg.Expired {
			return
		}
		delete(rg.Members, m.UserID)

		log.Printf("removing %s from raidgroup %s", m.UserID, rg.String())
		raid.SendUpdate(s)
		bs.dirty = true
		return
	}
}

func (bs *BotState) messageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	bs.mut.Lock()
	activemsg, activemsgok := bs.activeMessages[m.ID]
	bs.mut.Unlock()

	if activemsgok {
		activemsg.OnMessageDelete(bs, s, m)
		bs.RemoveActiveMessage(m.ID)
		return
	}

	bs.mut.Lock()
	raid, raidok := bs.Raids[m.ID]
	bs.mut.Unlock()
	if raidok {
		log.Printf("Deleting raid %s", raid.String())
		for _, rg := range raid.Groups {
			rg.Cancel(s)
		}
		bs.mut.Lock()
		delete(bs.Raids, m.Message.ID)
		bs.dirty = true
		bs.mut.Unlock()
	}

}

func (bs *BotState) userChannel(s *discordgo.Session, userID string) (string, error) {
	chanId, ok := bs.channelCache[userID]
	if !ok {
		userchan, err := s.UserChannelCreate(userID)
		log.Printf("created user channel for %s -> %s", userchan.Name, userchan.ID)
		if err != nil {
			return "", err
		}
		bs.channelCache[userID] = userchan.ID
		chanId = userchan.ID
	}
	return chanId, nil
}

func (bs *BotState) messageReactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	ch, _ := s.Channel(m.ChannelID)
	u, _ := s.User(m.UserID)

	if m.UserID == s.State.User.ID {
		return
	}

	log.Printf("reaction add: %s %s %s %s(%s)\n", ch.Name, m.MessageID, u.Username, m.Emoji.ID, m.Emoji.Name)

	if m.Emoji.Name == "⏰" {
		bs.mut.Lock()
		defer bs.mut.Unlock()

		raid, ok := bs.Raids[m.MessageID]
		if !ok {
			log.Print("...not raid")
			return
		}
		log.Print(raid.String())

		// remove the reaction once processed
		s.MessageReactionRemove(m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)

		ch, err := bs.userChannel(s, m.UserID)
		if err != nil {
			log.Print(err)
			return
		}
		_, err = s.ChannelMessageSend(ch, fmt.Sprintf("Adding a raid group for %s; what time?",
			raid.Gym.Name))
		if err != nil {
			log.Println(err)
			return
		}

		bs.channelCallbacks[ch] = func(s *discordgo.Session, privm *discordgo.MessageCreate) {
			if privm.Author.ID != m.UserID {
				log.Printf("??? private chat w/ %s but got userid %s?", m.UserID, privm.Author.ID)
				return
			}
			// parse the time
			log.Printf("got time from %s for raid %s: %s", privm.Author.Username, raid.String(), privm.Content)
			t, err := fuzzyTime(privm.Content, time.Now())
			if err != nil {
				s.ChannelMessageSend(privm.ChannelID, "Couldn't understand time "+privm.Content)
				log.Printf("can't parse time %s: %s", privm.Content, err)
				return
			}
			if t.Before(time.Now()) {
				s.ChannelMessageSend(privm.ChannelID, fmt.Sprintf(
					"%s is in the past!", t.Format("3:04 PM")))
			}
			if t.After(raid.EndTime) {
				s.ChannelMessageSend(privm.ChannelID, fmt.Sprintf(
					"%s is after the raid ends (at %s)!",
					t.Format("3:04 PM"), raid.EndTime.Format("3:04PM")))
				return
			}

			rg := raid.AddGroup(t, s)
			bs.dirty = true
			s.ChannelMessageSend(privm.ChannelID, "Got it! Created "+rg.String())
			// once a successful interaction has occurred, remove this callback
			delete(bs.channelCallbacks, privm.ChannelID)
		}

		return
	}

	if m.Emoji.Name[1:] == boxEmoji {
		n := m.Emoji.Name[0] - '1'

		bs.mut.Lock()
		defer bs.mut.Unlock()

		raid, ok := bs.Raids[m.MessageID]
		if !ok {
			return
		}

		if n < 0 || n >= uint8(len(raid.Groups)) {
			return
		}
		rg := raid.Groups[n]
		if rg.Expired {
			return
		}
		rg.Members[m.UserID] = 1

		log.Printf("adding %s to raidgroup %s", m.UserID, rg.String())
		raid.SendUpdate(s)
		bs.dirty = true
		return
	}

	// add/subtract extras
	if m.Emoji.Name == "➕" || m.Emoji.Name == "➖" {
		plus := m.Emoji.Name == "➕"
		bs.mut.Lock()
		defer bs.mut.Unlock()

		raid, ok := bs.Raids[m.MessageID]
		if !ok {
			return
		}

		// remove the reaction once processed
		s.MessageReactionRemove(m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)

		dirty := false
		for _, rg := range raid.Groups {
			if rg.Expired {
				continue
			}
			if _, ok := rg.Members[m.UserID]; ok {
				if plus {
					rg.Members[m.UserID]++
					dirty = true
				} else if rg.Members[m.UserID] > 1 {
					rg.Members[m.UserID]--
					dirty = true
				}
			}
		}
		if dirty {
			raid.SendUpdate(s)
		}
	}

	// all other custom emojis
	if m.Emoji.ID != "" {
		bs.mut.Lock()
		defer bs.mut.Unlock()
		raid, ok := bs.Raids[m.MessageID]
		if !ok {
			return
		}

		raid.Emoji = "<:" + m.Emoji.Name + ":" + m.Emoji.ID + "> "
		log.Printf("changing raid emoji: %s", raid.String())
		raid.SendUpdate(s)
		bs.dirty = true

	}
}

func (bs *BotState) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	bs.mut.Lock()
	if cb, ok := bs.channelCallbacks[m.ChannelID]; ok {
		bs.mut.Unlock()
		cb(s, m)
		return
	}
	bs.mut.Unlock()

	log.Printf("%s %s %s(%s): %s\n", m.Timestamp, m.ChannelID, m.Author.Username,
		m.Author.Email, m.ContentWithMentionsReplaced())

	if len(m.Content) > len(commandLeader) && m.Content[:len(commandLeader)] == commandLeader {
		bs.maybeProcessCommand(s, m)
	}
}

func (bs *BotState) maybeProcessCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	splitMsg := strings.SplitN(m.Content[len(commandLeader):], " ", 2)
	if len(splitMsg) == 0 {
		return
	}

	switch splitMsg[0] {
	case "info":
		bs.infoCommand(s, m, splitMsg[1])
	case "raid":
		bs.raidCommand(s, m, splitMsg[1])
	case "raidhelp":
		_, err := s.ChannelMessageSend(m.ChannelID, "Syntax:\n"+
			"`!info <gym name>` - get gym name and location\n"+
			"`!raid <pokemon> <gym name> ends/hatches [at 10:00pm/in 1h20m] [starts at 9:45pm]` - start a raid\n"+
			"Gym names are free-form text, fuzzy matched. Use !info to check whether I have the right one.\n"+
			"Editing or deleting your message requesting the raid will edit / cancel the raid.")
		if err != nil {
			log.Print(err)
		}
	case "dumpstate":
		m, err := json.Marshal(&bs)
		if err != nil {
			log.Print(err)
		}
		log.Print(string(m))
	case "scan":
		bs.scanCommand(s, m, splitMsg[1])
	}
}

func (bs *BotState) messageEdit(s *discordgo.Session, m *discordgo.MessageUpdate) {
	bs.mut.Lock()
	if msg, ok := bs.activeMessages[m.ID]; ok {
		bs.mut.Unlock()
		msg.OnMessageEdit(bs, s, m)
	} else {
		bs.mut.Unlock()
	}
}

// add embedded map to discord message
func addGymEmbed(g *gymdb.Gym, msg *discordgo.MessageSend) {
	msg.Embed = &discordgo.MessageEmbed{
		Title: g.Name,
		URL:   fmt.Sprintf("https://www.google.com/maps/?q=%f,%f", g.Latitude, g.Longitude),
		Image: &discordgo.MessageEmbedImage{
			// URL: g.ImageUrl,
			URL: fmt.Sprintf("https://maps.googleapis.com/maps/api/staticmap?center=%f,%f&markers=%f,%f&size=300x200&zoom=14",
				g.Latitude, g.Longitude, g.Latitude, g.Longitude),
			Width:  300,
			Height: 200,
		},
	}
}
