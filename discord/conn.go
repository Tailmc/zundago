package discord

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"
	"zundago/socket"
)

type Session struct {
	sock      *sock
	Cached    bool
	Presence  Presence
	Listeners Listeners
	Commands  []Command
}

type sock struct {
	bot     *Bot
	lock    bool
	seq     int
	intt    int
	mem     bool
	sent    int64
	ack     int64
	lat     int64
	pres    Presence
	sec     string
	que     []Command
	list    Listeners
	lazy    map[string]bool
	shard   Sharding
	runtime Runtime
	conn    *socket.Conn
	err     chan error
}

type glob struct {
	Guilds   map[string]*Guild
	Channels map[string]*Channel
	Voices   map[string]*Voice
}

type msg struct {
	Op int             `json:"op"`
	T  string          `json:"t"`
	S  int             `json:"s"`
	D  json.RawMessage `json:"d"`
}

type hello struct {
	HeartbeatInterval float64 `json:"heartbeat_interval"`
}

func (sock *sock) start() error {

	req, err := http.NewRequest(http.MethodGet, HOST+"gateway/bot", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+sock.sec)

	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		err = new(Error)
		json.NewDecoder(res.Body).Decode(err)
		return err
	}

	sharding := new(Sharding)
	err = json.NewDecoder(res.Body).Decode(sharding)
	if err != nil {
		return err
	}

	sharding.URL = sharding.URL + "?v=10&encoding=json"
	sock.shard = *sharding

	sock.conn, err = socket.Dial(sharding.URL)
	if err != nil {
		return err
	}

	go sock.event()

	sock.err = make(chan error, 1)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	select {
	case err := <-sock.err:
		return err
	case <-sig:
		return sock.end(1000)
	}
}

func (sock *sock) end(code int) error {

	for _, voice := range Global.Voices {
		sock.disconnect(voice.GuildId)
	}

	if sock.conn != nil {
		err := sock.conn.WriteClose(code)
		if err != nil {
			return err
		}

		time.Sleep(time.Second)

		err = sock.conn.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (sock *sock) connect(guild string, channel string, mute bool, deaf bool) (*Voice, error) {

	_, ok := Global.Voices[guild]
	if ok {
		err := sock.disconnect(guild)
		if err != nil {
			return nil, err
		}
	}

	voice := &Voice{
		ChannelId: channel,
		GuildId:   guild,
		mute:      mute,
		deaf:      deaf,
		ready:     make(chan bool, 1),
		server:    make(chan *VoiceServerUpdate, 1),
		state:     make(chan *VoiceState, 1),
	}

	dat := map[string]interface{}{
		"guild_id":   guild,
		"channel_id": channel,
		"self_mute":  mute,
		"self_deaf":  deaf,
	}

	err := sock.conn.WriteJSON(map[string]interface{}{"op": 4, "d": dat})
	if err != nil {
		return nil, err
	}

	Global.Voices[guild] = voice

	state, ok := <-voice.state
	if !ok {
		return nil, errors.New("voiceStateUpdate channel closed")
	}

	update, ok := <-voice.server
	if !ok {
		return nil, errors.New("voiceServerUpdate channel closed")
	}

	conn, err := socket.Dial("wss://" + update.Endpoint + "/?v=4")
	if err != nil {
		return nil, err
	}

	voice.conn = conn

	dat = map[string]interface{}{
		"server_id":  voice.GuildId,
		"user_id":    state.UserID,
		"session_id": state.SessionID,
		"token":      update.Token,
	}

	err = voice.conn.WriteJSON(map[string]interface{}{"op": 0, "d": dat})
	if err != nil {
		return nil, err
	}

	go voice.event()

	select {
	case <-voice.ready:
		return voice, nil
	case err := <-voice.err:
		return nil, err
	}
}

func (sock *sock) disconnect(guild string) error {

	voice, ok := Global.Voices[guild]

	if !ok {
		return errors.New("not connected to voice channels")
	}

	delete(Global.Voices, voice.GuildId)

	dat := map[string]interface{}{
		"guild_id":   guild,
		"channel_id": nil,
		"self_deaf":  true,
		"self_mute":  true,
	}

	err := sock.conn.WriteJSON(map[string]interface{}{"op": 4, "d": dat})
	if err != nil {
		return err
	}

	if voice.udp != nil {
		err := voice.udp.Close()
		if err != nil {
			return err
		}
	}

	if voice.conn != nil {
		err := voice.conn.WriteClose(1000)
		if err != nil {
			return err
		}

		time.Sleep(time.Second)

		err = voice.conn.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (sock *sock) event() {
	for {
		msg := new(msg)

		err := sock.conn.ReadJSON(msg)
		if errors.Is(err, io.EOF) {
			sock.err <- io.ErrUnexpectedEOF
			return
		} else if err != nil {
			sock.err <- err
			return
		}
		sock.seq = msg.S

		switch msg.T {
		case ready:
			runtime := new(Runtime)

			err := json.Unmarshal(msg.D, runtime)
			if err != nil {
				sock.err <- err
				return
			}
			sock.runtime = *runtime

			sock.lazy = make(map[string]bool, len(runtime.Guilds))
			for _, guild := range runtime.Guilds {
				sock.lazy[guild.Id] = true
			}

			bodies := make([]map[string]interface{}, len(sock.que))
			for idx, cmd := range sock.que {

				body := map[string]interface{}{
					"name":          cmd.Name,
					"description":   cmd.Description,
					"dm_permission": cmd.DMPermission,
				}
				body["type"] = 1

				ln := len(cmd.Options) + len(cmd.Subcommands) + len(cmd.SubcommandGroups)
				builds := make([]map[string]interface{}, ln)

				var pos int
				for idx := range cmd.Options {
					builds[pos] = cmd.Options[idx].build()
					pos++
				}

				for idx := range cmd.Subcommands {
					builds[pos] = cmd.Subcommands[idx].build()
					pos++
				}

				for idx := range cmd.SubcommandGroups {
					builds[pos] = cmd.SubcommandGroups[idx].build()
					pos++
				}

				body["options"] = builds

				def := strconv.Itoa(1 << 11)
				if len(cmd.Permissions) > 0 {
					var perm int
					for idx := range cmd.Permissions {
						perm |= int(cmd.Permissions[idx])
					}
					def = strconv.Itoa(perm)
				}

				body["default_member_permissions"] = def

				bodies[idx] = body
			}

			route := fmt.Sprintf("applications/%s/commands", runtime.Application.Id)

			buf := new(bytes.Buffer)

			err = json.NewEncoder(buf).Encode(bodies)
			if err != nil {
				sock.err <- err
				return
			}

			req, err := http.NewRequest(http.MethodPut, HOST+route, buf)
			if err != nil {
				sock.err <- err
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bot "+sock.sec)

			res, err := client.Do(req)
			if err != nil {
				sock.err <- err
				return
			}

			defer res.Body.Close()
			if res.StatusCode/100 != 2 {
				err = new(Error)
				json.NewDecoder(res.Body).Decode(err)
				sock.err <- err
				return
			}

			_, err = io.Copy(io.Discard, res.Body)
			if err != nil {
				sock.err <- err
				return
			}

			sock.bot = &runtime.User
			sock.bot.Latency = sock.lat
			sock.lock = false

			if sock.list.Ready != nil {
				go sock.list.Ready(sock.bot)
			}

		case guildCreate:
			guild := new(Guild)

			err := json.Unmarshal(msg.D, guild)
			if err != nil {
				sock.err <- err
				return
			}

			guild.ClientId = sock.bot.Id
			Global.Guilds[guild.Id] = guild

			for idx := range guild.Channels {
				Global.Channels[guild.Channels[idx].Id] = &guild.Channels[idx]
			}

			if sock.mem {
				dat := map[string]interface{}{
					"guild_id": guild.Id,
					"query":    "",
					"limit":    0,
				}

				err := sock.conn.WriteJSON(map[string]interface{}{"op": 8, "d": dat})
				if err != nil {
					sock.err <- err
					return
				}
			}

			if !sock.lazy[guild.Id] && sock.list.GuildCreate != nil {
				go sock.list.GuildCreate(sock.bot, guild)
			}

		case guildUpdate:

			guild := new(Guild)

			err := json.Unmarshal(msg.D, guild)
			if err != nil {
				sock.err <- err
				return
			}

			guild.ClientId = sock.bot.Id
			Global.Guilds[guild.Id] = guild

			for idx := range guild.Channels {
				Global.Channels[guild.Channels[idx].Id] = &guild.Channels[idx]
			}

		case guildDelete:

			guild := new(Guild)

			err := json.Unmarshal(msg.D, guild)
			if err != nil {
				sock.err <- err
				return
			}

			if sock.list.GuildDelete != nil {
				go sock.list.GuildDelete(sock.bot, guild)
			}

		case guildMembersChunk:
			guild := new(Guild)

			err := json.Unmarshal(msg.D, guild)
			if err != nil {
				sock.err <- err
				return
			}

			guild = Global.Guilds[guild.Id]
			for _, member := range guild.Members {
				if member.User.Id == guild.ClientId {
					guild.Me = member
					break
				}
			}

		case messageCreate:
			if sock.lock {
				break
			}

			if sock.list.MessageCreate != nil {
				message := new(Message)

				err := json.Unmarshal(msg.D, message)
				if err != nil {
					sock.err <- err
					return
				}

				go sock.list.MessageCreate(sock.bot, message)
			}

		case interactionCreate:
			if sock.lock {
				break
			}

			if sock.list.InteractionCreate != nil {
				interaction := new(Interaction)

				err := json.Unmarshal(msg.D, interaction)
				if err != nil {
					sock.err <- err
					return
				}

				if guild, ok := Global.Guilds[interaction.GuildId]; ok {
					interaction.Guild = *guild
				}

				if channel, ok := Global.Channels[interaction.ChannelId]; ok {
					interaction.Channel = *channel
				}

				go sock.list.InteractionCreate(sock.bot, interaction)
			}

		case voiceServerUpdate:
			if sock.lock {
				break
			}

			update := new(VoiceServerUpdate)

			err := json.Unmarshal(msg.D, update)
			if err != nil {
				sock.err <- err
				return
			}

			if voice, ok := Global.Voices[update.GuildID]; ok {
				voice.server <- update
			}

		case voiceStateUpdate:
			if sock.lock {
				break
			}

			state := new(VoiceState)

			err := json.Unmarshal(msg.D, state)
			if err != nil {
				sock.err <- err
				return
			}

			if voice, ok := Global.Voices[state.GuildID]; ok {
				if state.UserID == sock.bot.Id {
					voice.state <- state
				}
			}

			if guild, ok := Global.Guilds[state.GuildID]; ok {

				if state.ChannelID != "" {
					guild.VoiceStates = append(guild.VoiceStates, *state)
				}

				mp := make(map[string]bool)

				if state.ChannelID == "" {
					mp = map[string]bool{state.UserID: true}
				}

				nw := make([]VoiceState, 0)
				for idx := range guild.VoiceStates {
					state := guild.VoiceStates[len(guild.VoiceStates)-idx-1]
					if !mp[state.UserID] {
						mp[state.UserID] = true
						nw = append(nw, state)
					}
				}
				guild.VoiceStates = nw

				if sock.list.VoiceStateUpdate != nil {
					go sock.list.VoiceStateUpdate(sock.bot, guild.VoiceStates)
				}
			}
		}

		switch msg.Op {
		case 1:
			err := sock.conn.WriteJSON(map[string]interface{}{"op": 1, "d": sock.seq})
			if err != nil {
				sock.err <- err
				return
			}

			sock.sent = time.Now().UnixMilli()

		case 7:

			voices := Global.Voices

			err := sock.end(1012)
			if err != nil {
				sock.err <- err
				return
			}

			sock.list.Ready = func(bot *Bot) {
				for guild, voice := range voices {
					voice, err := sock.connect(guild, voice.ChannelId, voice.mute, voice.deaf)
					if err != nil {
						continue
					}
					Global.Voices[guild] = voice
				}
			}

			err = sock.start()
			if err != nil {
				sock.err <- err
				return
			}

		case 9:
			err := sock.ident()
			if err != nil {
				sock.err <- err
				return
			}

		case 10:
			err := sock.ident()
			if err != nil {
				sock.err <- err
				return
			}

			hello := new(hello)
			err = json.Unmarshal(msg.D, hello)
			if err != nil {
				sock.err <- err
				return
			}

			go func() {
				for {
					err := sock.conn.WriteJSON(map[string]interface{}{"op": 1, "d": sock.seq})
					if err != nil {
						sock.err <- err
						return
					}

					sock.sent = time.Now().UnixMilli()
					time.Sleep(time.Millisecond * time.Duration(hello.HeartbeatInterval))
				}
			}()

		case 11:
			sock.ack = time.Now().UnixMilli()
			sock.lat = sock.ack - sock.sent
			if sock.bot != nil {
				sock.bot.Latency = sock.lat
			}
		}
	}
}

func (sock *sock) ident() error {
	ident := map[string]interface{}{
		"token":   sock.sec,
		"intents": sock.intt,
	}
	props := map[string]string{
		"os":      "linux",
		"browser": "discord",
		"device":  "discord",
	}
	if sock.pres.Activity.Name != "" {
		pres := make(map[string]interface{})
		if sock.pres.Since != 0 {
			pres["since"] = sock.pres.Since
		}

		if sock.pres.Status != "" {
			pres["status"] = sock.pres.Status
		}

		if sock.pres.AFK {
			pres["afk"] = true
		}

		actv := make(map[string]interface{})

		actv["type"] = sock.pres.Activity.Type

		if sock.pres.Activity.Name != "" {
			actv["name"] = sock.pres.Activity.Name
		}

		if sock.pres.Activity.URL != "" {
			actv["url"] = sock.pres.Activity.URL
		}

		pres["activities"] = []map[string]interface{}{actv}
		ident["presence"] = pres
	}

	if sock.pres.OnMobile {
		props["browser"] = "Discord iOS"
	}

	ident["properties"] = props

	err := sock.conn.WriteJSON(map[string]interface{}{"op": 2, "d": ident})
	if err != nil {
		return err
	}

	return nil
}
