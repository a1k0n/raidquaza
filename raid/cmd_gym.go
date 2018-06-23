package raid

import (
	"github.com/bwmarrin/discordgo"
	"strings"
	"raidquaza/util"
	"log"
	"fmt"
)

func (bs *BotState) gymCommand(s *discordgo.Session, m *discordgo.MessageCreate, query string) {
	// usage:
	//  - !gym new <lat,lon> Gym Name
	//  - !gym edit <query> name New Name
	//  - !gym edit <query> location lat,lon
	//  - !gym remove <query>
	//  - !gym undo (?)
	tokens := strings.Split(query, " ")
	switch tokens[0] {
	case "new":
		if len(tokens) < 3 {
			s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> need gym lat/lon and name")
			return
		}
		lat, lon, n, err := util.ParseLatLong(tokens[1:])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> can't parse your lat/lon; example: -37.123,121.85")
			return
		}
		gym, err := bs.gymdb.AddGym(lat, lon, strings.Join(tokens[1+n:], " "))
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> "+err.Error())
			log.Print("AddGym error: ", err.Error())
			return
		}
		messageData := discordgo.MessageSend{}
		messageData.Content = fmt.Sprintf("<@%s> New gym added!\n[gym `%s`] %s | %s",
			m.Author.ID, gym.Id, gym.Name, gym.StreetAddr)
		addGymEmbed(gym, &messageData)
		s.ChannelMessageSendComplex(m.ChannelID, &messageData)
	case "remove":
		gs, _ := bs.gymdb.GetGyms(query, 1.0)
		if len(gs) == 0 {
			s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> couldn't find a matching gym")
			return
		}
		if len(gs) != 1 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> Which gym did you mean?\n%s",
				m.Author.ID, strings.Join(formatGymMatches(gs, nil), "\n")))
			return
		}
		err := bs.gymdb.RemoveGym(gs[0])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> error: "+err.Error())
			return
		}
		s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> gym deleted: "+gs[0].String())
	case "edit":
		q := strings.Split(query, " ")
		var gymquery []string
		var newname []string
		var newloc []string
		for i := len(q) - 1; i >= 0; i-- {
			if q[i] == "name" {
				gymquery = q[:i]
				newname = q[i+1:]
			} else if q[i] == "location" {
				gymquery = q[:i]
				newloc = q[i+1:]
			}
		}
		if gymquery == nil {
			s.ChannelMessageSend(m.ChannelID, "<@" + m.Author.ID+
				"> use `!gym edit <gym name/id> name <new name>`\n"+
				" or `!gym edit <gym name/id> location <lat,lon>")
			return
		}
		gs, _ := bs.gymdb.GetGyms(strings.Join(gymquery, " "), 1.0)
		if len(gs) == 0 {
			s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> couldn't find a matching gym")
			return
		}
		if len(gs) != 1 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s> Which gym did you mean?\n%s",
				m.Author.ID, strings.Join(formatGymMatches(gs, nil), "\n")))
			return
		}
		if newname != nil {
			oldName := gs[0].Name
			err := bs.gymdb.RenameGym(gs[0], strings.Join(newname, " "))
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> "+err.Error())
				return
			}
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<%s> Renamed `%s` to `%s`!",
				m.Author.ID, oldName, gs[0].Name))
		}
		if newloc != nil {
			lat, lon, _, err := util.ParseLatLong(newloc)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> couldn't understand the new location")
				return
			}
			oldLoc := fmt.Sprintf("%f,%f (%s)", gs[0].Latitude, gs[0].Longitude, gs[0].StreetAddr)
			err = bs.gymdb.MoveGym(gs[0], lat, lon)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> "+err.Error())
				return
			}
			newLoc := fmt.Sprintf("%f,%f (%s)", gs[0].Latitude, gs[0].Longitude, gs[0].StreetAddr)
			s.ChannelMessageSend(m.ChannelID, "<@" + m.Author.ID + "> Moved "+
				gs[0].Name+ " from "+ oldLoc+ " to "+ newLoc)
		}
	case "save": // undocumented
		log.Print("Resaving gymdb")
		err := bs.gymdb.UpdateDiskDB()
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> "+err.Error())
			return
		}
		s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> gym DB saved.")
	}
}
