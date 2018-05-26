package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"strings"
	"raidquaza/gymdb"
	"time"
	"sort"
	"sync"
	"encoding/json"
	"strconv"
)

const snapshotPath = "rqdata.json"

type Raid struct {
	Gym       *gymdb.Gym   `json:"gym"`
	What      string       `json:"what"`
	Emoji     string       `json:"emoji"` // latest reaction emoji, can indicate which pokemon
	EndTime   time.Time    `json:"end_time"`
	MessageID string       `json:"msg_id"`     // discord pinned message id
	ChannelID string       `json:"channel_id"` // discord channel pinned in
	Groups    []*RaidGroup `json:"groups"`
}

var boxEmoji = string([]byte{226, 131, 163})

func (r *Raid) genMessage() string {
	clockMsg := ""
	if len(r.Groups) == 0 {
		clockMsg = "\nClick ⏰ to add a raid group time."
	} else {
		clockMsg = "\nClick 🔢 to join group, ⏰ to add new time."
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
	s.ChannelMessageEdit(r.ChannelID, r.MessageID, r.genMessage())
}

type RaidGroup struct {
	raid      *Raid
	number    int
	StartTime time.Time      `json:"start_time"`
	Members   map[string]int `json:"members"` // discord userid set
	Expired   bool           `json:"expired"`
}

func (rg *RaidGroup) String() string {
	return fmt.Sprintf("%s%s raid at %s starting %s, ends %s", rg.raid.Emoji, rg.raid.What,
		rg.raid.Gym.Name, rg.StartTime.Format("3:04 PM"),
		rg.raid.EndTime.Format("3:04 PM"))
}

func (rg *RaidGroup) Total() int {
	total := 0
	for _, n := range rg.Members {
		total += n
	}
	return total
}

func (rg *RaidGroup) genMessage(n int) string {
	startTime := rg.StartTime.Format("3:04 PM")
	strikeThru := ""
	if rg.Expired {
		strikeThru = "~~"
	}
	return fmt.Sprintf("%d%s %s**%s** | %d attending: %s%s",
		n, boxEmoji, strikeThru, startTime, rg.Total(), rg.Mentions(), strikeThru)
}

func (rg *RaidGroup) Mentions() string {
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

func (r *Raid) AddGroup(startTime time.Time, s *discordgo.Session) *RaidGroup {
	n := len(r.Groups) + 1
	rg := &RaidGroup{
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

type BotState struct {
	emojiMap     map[string]string // emoji name -> emoji id
	channelCache map[string]string // userid -> privmsg channel id
	gymdb        *gymdb.GymDB

	Raids map[string]*Raid `json:"raids"` // message id -> raid

	channelCallbacks map[string]func(*discordgo.Session, *discordgo.MessageCreate)

	dirty bool

	// giant global lock
	mut sync.Mutex
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
	for _, raid := range bs.Raids {
		for _, rg := range raid.Groups {
			rg.raid = raid
		}
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

		if t.After(raid.EndTime) {
			raid.Expire(s)
			delete(bs.Raids, k)
			bs.dirty = true
		} else if needUpdate {
			raid.SendUpdate(s)
		}
	}
}

var botState BotState

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

	botState = BotState{
		emojiMap:     make(map[string]string),
		channelCache: make(map[string]string),
		gymdb:        gymdb.NewGymDB("gymdb/gyms.txt"),

		Raids:            make(map[string]*Raid),
		channelCallbacks: make(map[string]func(*discordgo.Session, *discordgo.MessageCreate)),
	}

	botState.Load(snapshotPath)

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(readyHandler)
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageDelete)
	dg.AddHandler(messageReactionAdd)
	dg.AddHandler(messageReactionRemove)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	go func(t *time.Ticker) {
		for range t.C {
			botState.ExpireOld(dg, time.Now())
			if botState.dirty {
				botState.Save(snapshotPath)
				botState.dirty = false
			}
		}
	}(ticker)

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	log.Println("Shutting down.")
	ticker.Stop()

	// Cleanly close down the Discord session.
	dg.Close()
}

func fuzzyTime(query string, beginTime time.Time) (*time.Time, error) {
	// interpret query time as the latest time before endTime
	t, err := time.Parse("3:04PM", strings.ToUpper(query))
	if err != nil {
		t, err = time.Parse("3:04 PM", strings.ToUpper(query))
		if err != nil {
			t, err = time.Parse("15:04", query)
			if err != nil {
				return nil, err
			}
		}
	}

	yy, mm, dd := beginTime.Date()
	h, m, _ := t.Clock()
	bh, _, _ := beginTime.Clock()
	if h < bh && h < 12 { // fix PM if not stated
		h += 12
	}
	result := time.Date(yy, mm, dd, h, m, 0, 0, beginTime.Location())
	return &result, nil
}

func readyHandler(s *discordgo.Session, r *discordgo.Ready) {
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
			botState.emojiMap[emoji.Name] = emoji.ID
		}
	}
	log.Println()
}

func messageReactionRemove(s *discordgo.Session, m *discordgo.MessageReactionRemove) {
	if m.UserID == s.State.User.ID {
		return
	}

	log.Printf("messageid %s %s reaction removed: %s(%s)", m.MessageID, m.UserID, m.Emoji.ID, m.Emoji.Name)

	if m.Emoji.Name[1:] == boxEmoji {
		n := m.Emoji.Name[0] - '1'
		botState.mut.Lock()
		defer botState.mut.Unlock()
		raid, ok := botState.Raids[m.MessageID]
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
		botState.dirty = true
		return
	}
}

func (rg *RaidGroup) Expire(s *discordgo.Session) {
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
	botState.dirty = true
}

func (rg *RaidGroup) Cancel(s *discordgo.Session) {
	log.Printf("%s deleted.", rg.String())
	if len(rg.Members) > 0 {
		s.ChannelMessageSend(rg.raid.ChannelID, fmt.Sprintf("%s %s was cancelled",
			rg.Mentions(), rg.String()))
	}

	botState.dirty = true
}

func (r *Raid) Expire(s *discordgo.Session) {
	log.Printf("%s expired.", r.String())
	s.ChannelMessageUnpin(r.ChannelID, r.MessageID)
	s.MessageReactionsRemoveAll(r.ChannelID, r.MessageID)
	delete(botState.Raids, r.MessageID)
	botState.dirty = true
}

func messageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	botState.mut.Lock()
	defer botState.mut.Unlock()

	if raid, ok := botState.Raids[m.Message.ID]; ok {
		log.Printf("Deleting raid %s", raid.String())
		for _, rg := range raid.Groups {
			rg.Cancel(s)
		}
		delete(botState.Raids, m.Message.ID)
		botState.dirty = true
	}
}

func userChannel(s *discordgo.Session, userID string) (string, error) {
	chanId, ok := botState.channelCache[userID]
	if !ok {
		userchan, err := s.UserChannelCreate(userID)
		log.Printf("created user channel for %s -> %s", userchan.Name, userchan.ID)
		if err != nil {
			return "", err
		}
		botState.channelCache[userID] = userchan.ID
		chanId = userchan.ID
	}
	return chanId, nil
}

func messageReactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	ch, _ := s.Channel(m.ChannelID)
	u, _ := s.User(m.UserID)

	if m.UserID == s.State.User.ID {
		return
	}

	log.Printf("reaction add: %s %s %s %s(%s)\n", ch.Name, m.MessageID, u.Username, m.Emoji.ID, m.Emoji.Name)

	if m.Emoji.Name == "⏰" {
		botState.mut.Lock()
		defer botState.mut.Unlock()

		raid, ok := botState.Raids[m.MessageID]
		if !ok {
			log.Print("...not raid")
			return
		}
		log.Print(raid.String())

		// remove the reaction once processed
		s.MessageReactionRemove(m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)

		ch, err := userChannel(s, m.UserID)
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

		botState.channelCallbacks[ch] = func(s *discordgo.Session, privm *discordgo.MessageCreate) {
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

			rg := raid.AddGroup(*t, s)
			botState.dirty = true
			s.ChannelMessageSend(privm.ChannelID, "Got it! Created "+rg.String())
			// once a successful interaction has occurred, remove this callback
			delete(botState.channelCallbacks, privm.ChannelID)
		}

		return
	}

	if m.Emoji.Name[1:] == boxEmoji {
		n := m.Emoji.Name[0] - '1'

		botState.mut.Lock()
		defer botState.mut.Unlock()

		raid, ok := botState.Raids[m.MessageID]
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
		botState.dirty = true
		return
	}

	// add/subtract extras
	if m.Emoji.Name == "➕" || m.Emoji.Name == "➖" {
		plus := m.Emoji.Name == "➕"
		botState.mut.Lock()
		defer botState.mut.Unlock()

		raid, ok := botState.Raids[m.MessageID]
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
		botState.mut.Lock()
		defer botState.mut.Unlock()
		raid, ok := botState.Raids[m.MessageID]
		if !ok {
			return
		}

		raid.Emoji = "<:" + m.Emoji.Name + ":" + m.Emoji.ID + "> "
		log.Printf("changing raid emoji: %s", raid.String())
		raid.SendUpdate(s)
		botState.dirty = true

	}
}

func expandPokemonAbbr(name string) string {
	if _, ok := botState.emojiMap["monsterface"]; ok {
		if len(name) == 2 && strings.ToLower(name[:1]) == "l" {
			m := "<:monsterface:" + botState.emojiMap["monsterface"] + ">"
			n, _ := strconv.Atoi(name[1:])
			return strings.Repeat(m, n)
		}
	} else {
		log.Printf("missing monsterface in emojimap")
	}
	return name
}

func formatGymMatches(gs []*gymdb.Gym) []string {
	var matches []string
	for _, g := range gs {
		matches = append(matches, fmt.Sprintf(
			"  [gym `%s`] %s %s <https://www.google.com/maps/?q=%f,%f>",
			g.Id, g.Name, g.StreetAddr, g.Latitude, g.Longitude))
	}
	return matches
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	botState.mut.Lock()
	defer botState.mut.Unlock()

	if cb, ok := botState.channelCallbacks[m.ChannelID]; ok {
		cb(s, m)
		return
	}

	ch, _ := s.Channel(m.ChannelID)
	log.Printf("%s %s %s(%s): %s\n", m.Timestamp, ch.Name, m.Author.Username,
		m.Author.Email, m.ContentWithMentionsReplaced())

	// If the message is "ping" reply with "Pong!"
	splitMsg := strings.Split(m.Content, " ")

	if len(splitMsg) == 0 {
		return
	}

	if splitMsg[0] == "!info" {
		query := strings.Join(splitMsg[1:], " ")
		gs := botState.gymdb.GetGyms(query, 0.5)
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
			matches = append(matches, formatGymMatches(gs)...)
			messageData.Content = strings.Join(matches, "\n")
		}

		_, err := s.ChannelMessageSendComplex(m.ChannelID, &messageData)
		if err != nil {
			log.Print(err)
		}
		return
	}

	// !raid ttar foo bar place ends at 4:00
	// !raid thing foo bar place ends in 23:51
	// !raid egg foo bar place ends in 15
	if splitMsg[0] == "!raid" {
		locEnd := len(splitMsg)
		if locEnd < 6 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> Use `!raid <pokemon> <location> ends [at/in] <time>`",
				m.Author.ID))
			return
		}
		for k, s := range splitMsg {
			if s == "ends" || s == "end" {
				locEnd = k
			}
		}
		what := splitMsg[1]
		gs := botState.gymdb.GetGyms(strings.Join(splitMsg[2:locEnd], " "), 1.0)
		if len(gs) == 0 { // no matches?
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> Couldn't find a gym matching \"%s\"",
				m.Author.ID, splitMsg[1:locEnd]))
			return
		}
		if len(gs) > 1 { // multiple potential matches?
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> Which gym did you mean?\n%s",
				m.Author.ID, strings.Join(formatGymMatches(gs), "\n")))
			return
		}

		// query matches just one gym:
		g := gs[0]

		if len(splitMsg) < locEnd+2 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> ends when? Try \"...ends at 4:00\" or \"...ends in 15m", m.Author.ID))
		}
		endTime := time.Now()
		if splitMsg[locEnd+1] == "at" {
			// try to parse the time
			timeQuery := strings.Join(splitMsg[locEnd+2:], " ")
			t, err := fuzzyTime(timeQuery, time.Now())
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> I didn't understand time %s", m.Author.ID, timeQuery))
				return
			}
			endTime = *t
		} else if splitMsg[locEnd+1] == "in" {
			t := splitMsg[locEnd+2]
			dur, err := time.ParseDuration(t)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> couldn't understand your duration: %s", m.Author.ID, err.Error()))
			}
			endTime = endTime.Add(dur)
		}

		raid := Raid{
			Gym:       g,
			What:      expandPokemonAbbr(what),
			EndTime:   endTime,
			ChannelID: m.ChannelID,
		}

		messageData := discordgo.MessageSend{
			Content: raid.genMessage(),
		}
		addGymEmbed(g, &messageData)

		msgId, err := s.ChannelMessageSendComplex(m.ChannelID, &messageData)
		if err != nil {
			log.Print(err)
			return
		}
		raid.MessageID = msgId.ID

		s.ChannelMessagePin(m.ChannelID, msgId.ID)

		err = s.MessageReactionAdd(m.ChannelID, msgId.ID, "⏰")
		if err != nil {
			s.ChannelMessageDelete(m.ChannelID, msgId.ID)
			log.Print(err)
			return
		}

		log.Printf("added [%s] %s", msgId.ID, raid.String())
		botState.Raids[msgId.ID] = &raid
		botState.dirty = true
		return
	}

	if m.Content == "!raidhelp" {
		_, err := s.ChannelMessageSend(m.ChannelID, "Syntax:\n"+
			"`!info <gym name>` - get gym name and location\n"+
			"`!raid <pokemon> <gym name> ends [at 10:00pm/in 1h20m]` - start a raid\n"+
			"Gym names are free-form text, fuzzy matched. Use !info to check whether I have the right one.")
		if err != nil {
			log.Print(err)
		}
	}

	if m.Content == "!dumpstate" {
		m, err := json.Marshal(&botState)
		if err != nil {
			log.Print(err)
		}
		log.Print(string(m))
	}
}
