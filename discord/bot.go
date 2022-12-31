package discord

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
	"zundago/dns"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	HOST = "https://discord.com/api/v10/"
)

const (
	ready             = "READY"
	messageCreate     = "MESSAGE_CREATE"
	guildCreate       = "GUILD_CREATE"
	guildUpdate       = "GUILD_UPDATE"
	guildDelete       = "GUILD_DELETE"
	interactionCreate = "INTERACTION_CREATE"
	guildMembersChunk = "GUILD_MEMBERS_CHUNK"
	voiceServerUpdate = "VOICE_SERVER_UPDATE"
	voiceStateUpdate  = "VOICE_STATE_UPDATE"
)

const (
	StringOption int = iota + 3
	IntOption
	BoolOption
	UserOption
	ChannelOption
	RoleOption
	MentionOption
	NumOption
	AttachmentOption
)

const (
	Playing ActivityType = iota
	Streaming
	Listening
	Watching
	Competing
)

const (
	Online    Status = "online"
	Idle      Status = "idle"
	Dnd       Status = "dnd"
	Invisible Status = "invisible"
	Offline   Status = "offline"
)

const (
	Guilds                      Intent = 1 << 0
	GuildMembers                Intent = 1 << 1
	GuildBans                   Intent = 1 << 2
	GuildEmojis                 Intent = 1 << 3
	GuildIntegrations           Intent = 1 << 4
	GuildWebhooks               Intent = 1 << 5
	GuildInvites                Intent = 1 << 6
	GuildVoiceStates            Intent = 1 << 7
	GuildPresences              Intent = 1 << 8
	GuildMessages               Intent = 1 << 9
	GuildMessageReactions       Intent = 1 << 10
	GuildMessageTyping          Intent = 1 << 11
	DirectMessages              Intent = 1 << 12
	DirectMessageReactions      Intent = 1 << 13
	DirectMessageTyping         Intent = 1 << 14
	MessageContent              Intent = 1 << 15
	GuildScheduledEvents        Intent = 1 << 16
	AutoModerationConfiguration Intent = 1 << 20
	AutoModerationExecution     Intent = 1 << 21
)

const (
	Ping int = iota + 1
	ApplicationCommand
	MessageComponent
	AutoComplete
	ModalSubmit
)

var (
	Global = &glob{
		Guilds:   make(map[string]*Guild),
		Channels: make(map[string]*Channel),
		Voices:   make(map[string]*Voice),
	}
	resolver = dns.New()
	client   = &http.Client{
		Transport: &http.Transport{
			DialContext:         resolver.Dial(),
			MaxIdleConnsPerHost: 50,
			MaxConnsPerHost:     25,
			IdleConnTimeout:     time.Second * 10,
		},
	}
)

func New(intent Intent) *Session {
	return &Session{
		sock: &sock{
			intt: int(intent),
			lock: true,
		},
	}
}

func (sess *Session) Start(tok string) error {

	if sess.Cached {
		sess.sock.mem = true
	}

	if sess.Presence.Activity.Name != "" {
		sess.sock.pres = sess.Presence
	}

	sess.sock.sec = tok

	sess.sock.que = sess.Commands
	sess.sock.list = sess.Listeners

	return sess.sock.start()
}

func (sess *Session) Connect(guild string, channel string, mute bool, deaf bool) (*Voice, error) {
	return sess.sock.connect(guild, channel, mute, deaf)
}

func (sess *Session) Disconnect(guild string) error {
	return sess.sock.disconnect(guild)
}

func (voice *Voice) Speak(speak bool) error {

	dat := map[string]interface{}{
		"speaking": speak,
		"delay":    0,
		"ssrc":     voice.two.SSRC,
	}

	err := voice.conn.WriteJSON(map[string]interface{}{"op": 5, "d": dat})
	if err != nil {
		return err
	}

	return nil
}

func (voice *Voice) event() {

	for {
		msg := new(msg)

		err := voice.conn.ReadJSON(msg)
		if err != nil {
			delete(Global.Voices, voice.GuildId)

			if voice.udp != nil {
				err := voice.udp.Close()
				if err != nil {
					voice.err <- err
					return
				}
			}

			if voice.conn != nil {
				err = voice.conn.Close()
				if err != nil {
					voice.err <- err
					return
				}
			}

			if errors.Is(err, io.EOF) {
				voice.err <- io.ErrUnexpectedEOF
				return
			}

			voice.err <- err
			return
		}

		switch msg.Op {
		case 2:
			err := json.Unmarshal(msg.D, &voice.two)
			if err != nil {
				voice.err <- err
				return
			}

			host := net.JoinHostPort(voice.two.IP, strconv.Itoa(voice.two.Port))

			adr, err := net.ResolveUDPAddr("udp", host)
			if err != nil {
				voice.err <- err
				return
			}

			voice.udp, err = net.DialUDP("udp", nil, adr)
			if err != nil {
				voice.err <- err
				return
			}

			ssrc := make([]byte, 70)
			binary.BigEndian.PutUint32(ssrc, voice.two.SSRC)

			_, err = voice.udp.Write(ssrc)
			if err != nil {
				voice.err <- err
				return
			}

			resp := make([]byte, 70)
			ln, _, err := voice.udp.ReadFromUDP(resp)
			if err != nil {
				voice.err <- err
				return
			}
			if ln < 70 {
				voice.err <- errors.New("received a small udp packet")
				return
			}

			var ip []byte
			for idx, byt := range resp {
				if byt == 0 {
					break
				}
				if idx+1 == len(resp) {
					voice.err <- errors.New("no termination character")
					return
				}
				ip = append(ip, byt)
			}

			port := binary.BigEndian.Uint16(resp[68:70])

			dat := map[string]interface{}{
				"protocol": "udp",
				"data": map[string]interface{}{
					"address": string(ip),
					"port":    port,
					"mode":    "xsalsa20_poly1305",
				},
			}

			err = voice.conn.WriteJSON(map[string]interface{}{"op": 1, "d": dat})
			if err != nil {
				voice.err <- err
				return
			}

		case 3:
		case 4:

			err := json.Unmarshal(msg.D, &voice.four)
			if err != nil {
				voice.err <- err
				return
			}

			if voice.Send == nil {
				voice.Send = make(chan []byte)
			}

			voice.ready <- true

			go voice.send()

		case 8:
			hello := new(hello)
			err := json.Unmarshal(msg.D, hello)
			if err != nil {
				voice.err <- err
				return
			}

			go func() {
				for {
					if voice.conn == nil {
						return
					}

					err = voice.conn.WriteJSON(map[string]interface{}{"op": 3, "d": time.Now().Unix()})
					if err != nil {
						voice.err <- err
						return
					}

					time.Sleep(time.Millisecond * time.Duration(hello.HeartbeatInterval))
				}
			}()
		}
	}
}

func (voice *Voice) send() {
	var seq uint16
	var timestamp uint32
	var nonce [24]byte

	head := make([]byte, 12)
	head[0] = 0x80
	head[1] = 0x78
	binary.BigEndian.PutUint32(head[8:], voice.two.SSRC)

	tick := time.NewTicker(time.Millisecond * 20)
	defer tick.Stop()
	for {
		opus, ok := <-voice.Send
		if !ok {
			return
		}

		binary.BigEndian.PutUint16(head[2:4], seq)
		seq++

		binary.BigEndian.PutUint32(head[4:8], timestamp)
		timestamp += 960

		copy(nonce[:], head)
		buf := secretbox.Seal(head, opus, &nonce, &voice.four.SecretKey)

		<-tick.C

		voice.udp.Write(buf)
	}
}

func (int *Interaction) Reply(resp *Response) error {
	route := fmt.Sprintf("interactions/%s/%s/callback", int.Id, int.Token)
	dat, err := resp.build()
	if err != nil {
		return err
	}

	body, boundary, err := multiPart(map[string]interface{}{"type": 4, "data": dat}, resp.Files)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, HOST+route, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

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

	_, err = io.Copy(io.Discard, res.Body)
	if err != nil {
		return err
	}

	return nil
}

func (int *Interaction) Defer(ephemeral bool) error {

	dat := make(map[string]interface{})
	dat["type"] = 6
	if int.Type == 2 {
		dat["type"] = 5
		if ephemeral {
			dat["data"] = map[string]interface{}{
				"flags": 1 << 6,
			}
		}
	}

	route := fmt.Sprintf("interactions/%s/%s/callback", int.Id, int.Token)

	body := new(bytes.Buffer)

	err := json.NewEncoder(body).Encode(dat)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, HOST+route, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

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

	_, err = io.Copy(io.Discard, res.Body)
	if err != nil {
		return err
	}

	return nil
}

func (int *Interaction) Edit(resp *Response) error {
	dat, err := resp.build()
	if err != nil {
		return err
	}

	var route string
	var method string

	if int.Type == 2 {
		route = fmt.Sprintf("webhooks/%s/%s/messages/@original", int.ApplicationId, int.Token)
		method = http.MethodPatch
	} else {
		route = fmt.Sprintf("interactions/%s/%s/callback", int.Id, int.Token)
		method = http.MethodPost
		dat = map[string]interface{}{"type": 7, "data": dat}
	}

	body, boundary, err := multiPart(dat, resp.Files)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, HOST+route, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

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

	_, err = io.Copy(io.Discard, res.Body)
	if err != nil {
		return err
	}

	return nil
}

func (int Interaction) Delete() error {
	route := fmt.Sprintf("webhooks/%s/%s/messages/@original", int.ApplicationId, int.Token)

	req, err := http.NewRequest(http.MethodDelete, HOST+route, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

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

	_, err = io.Copy(io.Discard, res.Body)
	if err != nil {
		return err
	}

	return nil
}

func multiPart(dat map[string]interface{}, fls []File) (io.Reader, string, error) {
	var buf bytes.Buffer

	wrt := multipart.NewWriter(&buf)

	jsn, err := json.MarshalIndent(dat, "", "  ")
	if err != nil {
		return nil, "", err
	}

	head := make(textproto.MIMEHeader)
	head.Set("Content-Disposition", `form-data; name="payload_json"`)
	head.Set("Content-Type", `application/json`)

	prt, err := wrt.CreatePart(head)
	if err != nil {
		return nil, "", err
	}

	_, err = prt.Write(jsn)
	if err != nil {
		return nil, "", err
	}

	esc := strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

	for idx := range fls {
		if fls[idx].Content == nil {
			return nil, "", errors.New("file (" + fls[idx].Name + ") is empty")
		}

		fl, err := wrt.CreateFormFile(
			fmt.Sprintf("files[%d]", idx),
			esc.Replace(fls[idx].Name),
		)
		if err != nil {
			return nil, "", err
		}

		_, err = fl.Write(fls[idx].Content)
		if err != nil {
			return nil, "", err
		}
	}

	wrt.Close()

	return &buf, wrt.Boundary(), nil
}
