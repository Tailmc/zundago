package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
	"zundago/discord"
	"zundago/dns"
	"zundago/ffmpeg"
	"zundago/ogg"
	"zundago/redis"
)

type vc struct {
	channelId string
	voice     *discord.Voice
	mutex     *sync.Mutex
	dict      []string
}

const (
	host  = "https://api.su-shiki.com/v2/voicevox/audio/"
	green = 0xa4d5ad
)

var (
	rgx      = regexp.MustCompile(`<?(a)?:?(\w{2,32}):(\d{17,19})>?|(?:https?|ftp):\/\/[\n\S]+`)
	def      = new(sync.Map)
	vcs      = new(sync.Map)
	resolver = dns.New()
	client   = &http.Client{
		Transport: &http.Transport{
			DialContext:         resolver.Dial(),
			MaxIdleConnsPerHost: 50,
			MaxConnsPerHost:     25,
			IdleConnTimeout:     time.Second * 10,
		},
	}
	choises = []discord.Choice{
		{
			Name:  "ずんだもん: ノーマル",
			Value: "3",
		}, {
			Name:  "ずんだもん: あまあま",
			Value: "1",
		}, {
			Name:  "ずんだもん: ツンツン",
			Value: "7",
		}, {
			Name:  "ずんだもん: セクシー",
			Value: "5",
		}, {
			Name:  "四国めたん: ノーマル",
			Value: "2",
		}, {
			Name:  "四国めたん: あまあま",
			Value: "0",
		}, {
			Name:  "四国めたん: ツンツン",
			Value: "6",
		}, {
			Name:  "四国めたん: セクシー",
			Value: "4",
		}, {
			Name:  "春日部つむぎ: ノーマル",
			Value: "8",
		}, {
			Name:  "雨晴はう: ノーマル",
			Value: "10",
		}, {
			Name:  "波音リツ: ノーマル",
			Value: "9",
		}, {
			Name:  "玄野武宏: ノーマル",
			Value: "11",
		}, {
			Name:  "白上虎太郎: ノーマル",
			Value: "12",
		}, {
			Name:  "青山龍星: ノーマル",
			Value: "13",
		}, {
			Name:  "冥鳴ひまり: ノーマル",
			Value: "14",
		}, {
			Name:  "九州そら: ノーマル",
			Value: "16",
		}, {
			Name:  "九州そら: あまあま",
			Value: "15",
		}, {
			Name:  "九州そら: ツンツン",
			Value: "18",
		}, {
			Name:  "九州そら: セクシー",
			Value: "17",
		}, {
			Name:  "九州そら: ささやき",
			Value: "19",
		}, {
			Name:  "モチノ・キョウコ: ノーマル",
			Value: "20",
		},
	}
)

func main() {

	err := os.MkdirAll("dict", 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	opn, err := os.Open("def.env")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer opn.Close()

	scn := bufio.NewScanner(opn)
	for scn.Scan() {
		txt := scn.Text()
		spl := strings.SplitN(txt, "=", 2)
		err := os.Setenv(spl[0], spl[1])
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	voicevox := os.Getenv("VOICEVOX")

	db := &redis.DB{
		URI:  os.Getenv("REDISURI"),
		Pass: os.Getenv("REDISPASS"),
	}

	db.Port, err = strconv.Atoi(os.Getenv("REDISPORT"))
	if err != nil {
		fmt.Println(err)
		return
	}

	err = db.Dial()
	if err != nil {
		fmt.Println(err)
		return
	}

	opn, err = os.Open("def.txt")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer opn.Close()

	scn = bufio.NewScanner(opn)
	for scn.Scan() {
		txt := scn.Text()
		if !strings.HasPrefix(txt, "#") {
			spl := strings.SplitN(txt, " ", 2)
			def.Store(spl[0], spl[1])
		}
	}

	sess := discord.New(discord.Guilds | discord.GuildMessages | discord.MessageContent | discord.GuildVoiceStates)

	sess.Cached = true

	sess.Presence = discord.Presence{
		Status: discord.Online,
		Activity: discord.Activity{
			Name: "/help",
			Type: discord.Streaming,
			URL:  "https://www.youtube.com/watch?v=4yVpklclxwU",
		},
	}

	sess.Listeners = discord.Listeners{
		Ready: func(bot *discord.Bot) {
			fmt.Println(bot.Username + "#" + bot.Discriminator)
		},
		InteractionCreate: func(bot *discord.Bot, interaction *discord.Interaction) {
			switch interaction.Type {
			case discord.Ping:
			case discord.ApplicationCommand:
				switch interaction.Data.Name {

				case "help":

					resp := &discord.Response{
						Content: ":question: ヘルプメニュー",
						Embeds: []discord.Embed{{Description: "/help - このメニューを表示\n" +
							"/ping - ping値を表示\n" +
							"/join - 読み上げを開始\n" +
							"/leave - 読み上げを終了\n" +
							"/switch - キャラクターを変更\n" +
							"/dict - 辞書を変更\n" +
							"/export - 辞書を出力", Color: green}},
					}
					interaction.Reply(resp)

				case "ping":

					resp := &discord.Response{
						Content: ":timer: レイテンシ",
						Embeds:  []discord.Embed{{Description: strconv.FormatInt(bot.Latency, 10) + "ms", Color: green}},
					}
					interaction.Reply(resp)

				case "join":

					err := interaction.Defer(false)
					if err != nil {
						return
					}

					var ok bool
					var voice *discord.Voice
					for _, state := range interaction.Guild.VoiceStates {
						if state.UserID == interaction.Author.User.Id {
							voice, err = sess.Connect(interaction.GuildId, state.ChannelID, false, true)
							ok = err == nil
							break
						}
					}

					if !ok {
						resp := &discord.Response{
							Content: ":red_circle: 失敗...",
							Embeds:  []discord.Embed{{Description: "ボイスチャンネルに接続できなかったのだ", Color: green}},
						}
						interaction.Edit(resp)
						return
					}

					dict := make([]string, 0)
					opn, err := os.Open(filepath.Join("dict", interaction.GuildId+".dict"))
					if err == nil {
						defer opn.Close()
						err := gob.NewDecoder(opn).Decode(&dict)
						if err != nil {
							resp := &discord.Response{
								Content: ":red_circle: 失敗...",
								Embeds:  []discord.Embed{{Description: "辞書の読み込みに失敗したのだ", Color: green}},
							}
							interaction.Edit(resp)
							return
						}
					}

					vc := &vc{interaction.ChannelId, voice, new(sync.Mutex), dict}
					vcs.Store(interaction.GuildId, vc)

					resp := &discord.Response{
						Content: ":green_circle: 成功!",
						Embeds:  []discord.Embed{{Description: "ボイスチャンネルに接続したのだ", Color: green}},
					}
					interaction.Edit(resp)

				case "leave":

					err := interaction.Defer(false)
					if err != nil {
						return
					}

					any, ok := vcs.Load(interaction.GuildId)
					if !ok {
						resp := &discord.Response{
							Content: ":red_circle: 失敗...",
							Embeds:  []discord.Embed{{Description: "ボイスチャンネルが見つからなかったのだ", Color: green}},
						}
						interaction.Edit(resp)
						return
					}

					vc := any.(*vc)

					err = sess.Disconnect(interaction.GuildId)
					if err != nil {
						resp := &discord.Response{
							Content: ":red_circle: 失敗...",
							Embeds:  []discord.Embed{{Description: "ボイスチャンネルから切断できなかったのだ", Color: green}},
						}
						interaction.Edit(resp)
						return
					}

					crt, err := os.Create(filepath.Join("dict", interaction.GuildId+".dict"))
					if err == nil {
						defer crt.Close()
						err := gob.NewEncoder(crt).Encode(vc.dict)
						if err != nil {
							resp := &discord.Response{
								Content: ":red_circle: 失敗...",
								Embeds:  []discord.Embed{{Description: "辞書の保存に失敗したのだ", Color: green}},
							}
							interaction.Edit(resp)
							return
						}
					}

					vcs.Delete(interaction.GuildId)

					resp := &discord.Response{
						Content: ":green_circle: 成功!",
						Embeds:  []discord.Embed{{Description: "ボイスチャンネルから切断したのだ", Color: green}},
					}
					interaction.Edit(resp)

				case "dict":

					err := interaction.Defer(false)
					if err != nil {
						return
					}

					any, ok := vcs.Load(interaction.GuildId)
					if !ok {
						resp := &discord.Response{
							Content: ":red_circle: 失敗...",
							Embeds:  []discord.Embed{{Description: "ボイスチャンネルが見つからなかったのだ", Color: green}},
						}
						interaction.Edit(resp)
						return
					}

					vc := any.(*vc)

					var old string
					var nw string
					for _, option := range interaction.Data.Options {
						if option.Name == "old" {
							old = option.Value.(string)
						} else {
							nw = option.Value.(string)
						}
					}

					vc.dict = append(vc.dict, old, nw)

					resp := &discord.Response{
						Content: ":green_circle: 成功!",
						Embeds:  []discord.Embed{{Description: old + "を" + nw + "として保存したのだ", Color: green}},
					}
					interaction.Edit(resp)

				case "switch":

					err := interaction.Defer(true)
					if err != nil {
						return
					}

					val := interaction.Data.Options[0].Value.(string)
					i32, err := strconv.Atoi(val)
					if err != nil || i32 > 20 || i32 < 0 {
						resp := &discord.Response{
							Content: ":red_circle: 失敗...",
							Embeds:  []discord.Embed{{Description: "データが無効な可能性があるのだ", Color: green}},
						}
						interaction.Edit(resp)
						return
					}

					err = db.Send("SET", interaction.Author.User.Id, val)
					if err != nil {
						resp := &discord.Response{
							Content: ":red_circle: 失敗...",
							Embeds:  []discord.Embed{{Description: "データの保存に失敗したのだ", Color: green}},
						}
						interaction.Edit(resp)
						return
					}

					res := db.Receive()
					if _, ok := res.(error); ok {
						resp := &discord.Response{
							Content: ":red_circle: 失敗...",
							Embeds:  []discord.Embed{{Description: "データの保存に失敗したのだ", Color: green}},
						}
						interaction.Edit(resp)
						return
					}

					for _, choise := range choises {
						if choise.Value.(string) == val {
							resp := &discord.Response{
								Content: ":green_circle: 成功!",
								Embeds:  []discord.Embed{{Description: choise.Name + "に設定したのだ", Color: green}},
							}
							interaction.Edit(resp)
							break
						}
					}

				case "export":

					err := interaction.Defer(true)
					if err != nil {
						return
					}

					opn, err := os.Open(filepath.Join("dict", interaction.GuildId+".dict"))
					if err != nil {
						defer opn.Close()
						resp := &discord.Response{
							Content: ":red_circle: 失敗...",
							Embeds:  []discord.Embed{{Description: "辞書の読み込みに失敗したのだ", Color: green}},
						}
						interaction.Edit(resp)
						return
					}

					var byt []byte
					var ext string
					switch interaction.Data.Options[0].Value.(string) {
					case "gob":

						buf := new(bytes.Buffer)
						_, err := io.Copy(buf, opn)
						if err != nil {
							resp := &discord.Response{
								Content: ":red_circle: 失敗...",
								Embeds:  []discord.Embed{{Description: "辞書の読み込みに失敗したのだ", Color: green}},
							}
							interaction.Edit(resp)
							return
						}
						byt, ext = buf.Bytes(), ".dict"

					case "json":

						dict := make([]string, 0)
						err := gob.NewDecoder(opn).Decode(&dict)
						if err != nil {
							resp := &discord.Response{
								Content: ":red_circle: 失敗...",
								Embeds:  []discord.Embed{{Description: "辞書の読み込みに失敗したのだ", Color: green}},
							}
							interaction.Edit(resp)
							return
						}

						byt, err = json.MarshalIndent(dict, "", "  ")
						if err != nil {
							resp := &discord.Response{
								Content: ":red_circle: 失敗...",
								Embeds:  []discord.Embed{{Description: "辞書の読み込みに失敗したのだ", Color: green}},
							}
							interaction.Edit(resp)
						}
						ext = ".json"
					}

					resp := &discord.Response{
						Content: ":green_circle: 成功!",
						Embeds:  []discord.Embed{{Description: "ファイルを添付したのだ", Color: green}},
						Files:   []discord.File{{Name: interaction.GuildId + ext, Content: byt}},
					}
					interaction.Edit(resp)

				}
			}
		},

		MessageCreate: func(bot *discord.Bot, msg *discord.Message) {

			if msg.Author.Bot || msg.Author.System {
				return
			}

			any, ok := vcs.Load(msg.GuildId)
			if !ok {
				return
			}

			vc := any.(*vc)
			if vc.channelId != msg.ChannelId {
				return
			}

			vc.mutex.Lock()
			defer vc.mutex.Unlock()

			con := msg.Content

			if !utf8.ValidString(con) {
				return
			}

			if len(con) >= 60 {
				con = string([]rune(con[:60])) + " 以下略"
			}
			words := []string{"\n", " "}

			for _, mention := range msg.Mentions {
				words = append(words, "<@"+mention.Id+">", "@"+mention.Username, "<@!"+mention.Id+">", "@"+mention.Username)
			}

			for _, mention := range msg.ChannelMentions {
				words = append(words, "<#"+mention.Id+">", "#"+mention.Name)
			}

			if guild, ok := discord.Global.Guilds[msg.GuildId]; ok {
				for _, mention := range msg.RoleMentions {
					for _, role := range guild.Roles {
						if role.Id == mention {
							words = append(words, "<@&"+role.Id+">", "@"+role.Name)
							break
						}
					}
				}
			}

			var prev int
			for idx, curr := range []rune(con) {
				low := unicode.ToLower(curr)
				if 'a' > low || 'z' < low {
					if idx <= prev {
						continue
					}
					word := string([]rune(con)[prev:idx])
					if any, ok := def.Load(strings.ToLower(word)); ok {
						words = append(words, word, any.(string))
					}
					prev = idx
				}
			}

			con = strings.NewReplacer(vc.dict...).Replace(con)
			con = strings.NewReplacer(words...).Replace(con)

			con = rgx.ReplaceAllString(con, "")

			if len(con) == 0 {
				return
			}

			err := db.Send("GET", msg.Author.Id)
			if err != nil {
				return
			}

			res := db.Receive()

			var speaker string
			switch res := res.(type) {
			case []byte:
				speaker = string(res)
			case error:
				if !errors.Is(res, redis.Nil) {
					return
				}
				speaker = "3"
			default:
				return
			}

			url := host + "?key=" + voicevox + "&speaker=" + speaker + "&text=" + url.QueryEscape(con)
			get, err := client.Get(url)
			if err != nil {
				return
			}

			defer get.Body.Close()
			if get.StatusCode != http.StatusOK {
				return
			}

			buf := bufio.NewReader(get.Body)

			cmd, err := ffmpeg.New(buf)
			if err != nil {
				return
			}

			vc.voice.Speak(true)
			defer vc.voice.Speak(false)

			out, err := cmd.Run()
			if err != nil {
				return
			}
			defer cmd.Process.Kill()

			dec := ogg.New(out)
			for {
				byt, err := dec.Decode()
				if err != nil {
					break
				}
				vc.voice.Send <- byt
			}
		},

		VoiceStateUpdate: func(bot *discord.Bot, voiceStates []discord.VoiceState) {

			if len(voiceStates) < 1 {
				return
			}

			any, ok := vcs.Load(voiceStates[0].GuildID)
			if !ok {
				return
			}

			vc := any.(*vc)

			for _, state := range voiceStates {
				if bot.Id != state.UserID || vc.voice.ChannelId != state.ChannelID || state.Member.User.Bot {
					continue
				}
				ok = false
				break
			}

			if ok {
				err := sess.Disconnect(vc.voice.GuildId)
				if err != nil {
					return
				}
				vcs.Delete(vc.voice.GuildId)
			}
		},
	}

	sess.Commands = []discord.Command{
		{
			Name:        "help",
			Description: "ヘルプメニューを表示するのだ",
		}, {
			Name:        "ping",
			Description: "ping値を返すのだ",
		}, {
			Name:        "join",
			Description: "読み上げを開始するのだ",
		}, {
			Name:        "leave",
			Description: "読み上げを終了するのだ",
		}, {
			Name:        "dict",
			Description: "辞書を変更するのだ",
			Options: []discord.Option{
				{
					Name:        "old",
					Type:        discord.StringOption,
					Description: "置き換える単語",
					MaxLength:   20,
					MinLength:   1,
					Required:    true,
				}, {
					Name:        "new",
					Type:        discord.StringOption,
					Description: "新しい単語",
					MaxLength:   20,
					MinLength:   1,
					Required:    true,
				},
			},
		}, {
			Name:        "switch",
			Description: "キャラクターを変更",
			Options: []discord.Option{
				{
					Name:        "speaker",
					Type:        discord.StringOption,
					Description: "新しいキャラクター",
					MaxLength:   20,
					MinLength:   1,
					Required:    true,
					Choices:     choises,
				},
			},
		}, {
			Name:        "export",
			Description: "辞書ファイルを出力するのだ",
			Options: []discord.Option{
				{
					Name:        "encoding",
					Type:        discord.StringOption,
					Description: "使うファイル形式",
					MaxLength:   20,
					MinLength:   1,
					Required:    true,
					Choices: []discord.Choice{
						{
							Name:  "Gob",
							Value: "gob",
						}, {
							Name:  "JSON",
							Value: "json",
						},
					},
				},
			},
		},
	}

	tok := os.Getenv("TOKEN")
	if tok == "" {
		fmt.Println(errors.New("discord auth token not set"))
		return
	}

	err = sess.Start(tok)
	if err != nil {
		fmt.Println(err)
		return
	}
}
