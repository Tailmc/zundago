package discord

import (
	"net"
	"strconv"
	"time"
	"zundago/socket"
)

type Intent int

type Bot struct {
	Id            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	MfaEnabled    bool   `json:"mfa_enabled"`
	Banner        string `json:"banner"`
	Color         int    `json:"accent_color"`
	Locale        string `json:"locale"`
	Verified      bool   `json:"verified"`
	Flags         int    `json:"flags"`
	PublicFlags   int    `json:"public_flags"`
	Latency       int64  `json:"latency"`
}

type Guild struct {
	Id                          string                   `json:"id"`
	Name                        string                   `json:"name"`
	Owner                       bool                     `json:"owner"`
	OwnerID                     string                   `json:"owner_id"`
	Permissions                 string                   `json:"permissions"`
	Region                      string                   `json:"region"`
	AfkChannelID                string                   `json:"afk_channel_id"`
	AfkTimeout                  int                      `json:"afk_timeout"`
	WidgetEnabled               bool                     `json:"widget_enabled"`
	WidgetChannelID             string                   `json:"widget_channel_id"`
	VerificationLevel           int                      `json:"verification_level"`
	DefaultMessageNotifications int                      `json:"default_message_notifications"`
	ExplicitContentFilter       int                      `json:"explicit_content_filter"`
	Emojis                      []Emoji                  `json:"emojis"`
	Features                    []string                 `json:"features"`
	MFALevel                    int                      `json:"mfa_level"`
	ApplicationID               string                   `json:"application_id"`
	SystemChannelID             string                   `json:"system_channel_id"`
	SystemChannelFlags          int                      `json:"system_channel_flags"`
	RulesChannelID              string                   `json:"rules_channel_id"`
	MaxPresences                int                      `json:"max_presences"`
	MaxMembers                  int                      `json:"max_members"`
	VanityURLCode               string                   `json:"vanity_url_code"`
	Description                 string                   `json:"description"`
	PremiumTier                 int                      `json:"premium_tier"`
	PremiumSubscriptionCount    int                      `json:"premium_subscription_count"`
	PreferredLocale             string                   `json:"preferred_locale"`
	PublicUpdatesChannelID      string                   `json:"public_updates_channel_id"`
	MaxVideoChannelUsers        int                      `json:"max_video_channel_users"`
	ApproximateMemberCount      int                      `json:"approximate_member_count"`
	ApproximatePresenceCount    int                      `json:"approximate_presence_count"`
	WelcomeScreen               map[string]interface{}   `json:"welcome_screen_enabled"`
	NSFWLevel                   int                      `json:"nsfw_level"`
	Stickers                    []map[string]interface{} `json:"stickers"`
	PremiumProgressBarEnabled   bool                     `json:"premium_progress_bar_enabled"`
	JoinedAT                    string                   `json:"joined_at"`
	Large                       bool                     `json:"large"`
	MemberCount                 int                      `json:"member_count"`
	VoiceStates                 []VoiceState             `json:"voice_states"`
	Presences                   []map[string]interface{} `json:"presences"`
	Threads                     []map[string]interface{} `json:"threads"`
	StageInstances              []map[string]interface{} `json:"stage_instances"`
	Unavailable                 bool                     `json:"unavailable"`
	GuildScheduledEvents        []map[string]interface{} `json:"guild_scheduled_events"`
	ClientId                    string                   `json:"-"`
	Icon                        string                   `json:"icon"`
	Banner                      string                   `json:"banner"`
	Splash                      string                   `json:"splash"`
	DiscoverySplash             string                   `json:"discovery_splash"`
	Me                          Member                   `json:"-"`
	Roles                       []Role                   `json:"roles"`
	Members                     []Member                 `json:"members"`
	Channels                    []Channel                `json:"channels"`
}

type Channel struct {
	Id                         string        `json:"id"`
	Type                       int           `json:"type"`
	GuildId                    string        `json:"guild_id"`
	Position                   int           `json:"position"`
	Overwrites                 []interface{} `json:"permission_overwrites"`
	Name                       string        `json:"name"`
	Topic                      string        `json:"topic"`
	NSFW                       bool          `json:"nsfw"`
	LastMessageId              string        `json:"last_message_id"`
	Bitrate                    int           `json:"bitrate"`
	UserLimit                  int           `json:"user_limit"`
	RateLimitPerUser           int           `json:"rate_limit_per_user"`
	Recipients                 []interface{} `json:"recipients"`
	Icon                       string        `json:"icon"`
	OwnerId                    string        `json:"owner_id"`
	ApplicationId              string        `json:"application_id"`
	ParentId                   string        `json:"parent_id"`
	LastPinTimestamp           string        `json:"last_pin_timestamp"`
	RTCRegion                  string        `json:"rtc_region"`
	VideoQualityMode           int           `json:"video_quality_mode"`
	MessageCount               int           `json:"message_count"`
	ThreadMetaData             interface{}   `json:"thread_metadata"`
	Member                     interface{}   `json:"member"`
	DefaultAutoArchiveDuration int           `json:"default_auto_archive_days"`
	Permissions                string        `json:"permissions"`
	Flags                      int           `json:"flags"`
	TotalMessages              int           `json:"total_messages"`
}

type User struct {
	Id            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Bot           bool   `json:"bot"`
	System        bool   `json:"system"`
	MfaEnabled    bool   `json:"mfa_enabled"`
	Color         int    `json:"accent_color"`
	Locale        string `json:"locale"`
	Verified      bool   `json:"verified"`
	Email         string `json:"email"`
	Flags         int    `json:"flags"`
	PremiumType   int    `json:"premium_type"`
	PublicFlags   int    `json:"public_flags"`
	Avatar        string `json:"avatar"`
	Banner        string `json:"banner"`
}

type Member struct {
	Nickname      string   `json:"nick"`
	AvatarHash    string   `json:"avatar"`
	JoinedAt      string   `json:"joined_at"`
	PremiumSince  string   `json:"premium_since"`
	Deaf          bool     `json:"deaf"`
	Mute          bool     `json:"mute"`
	Pending       bool     `json:"pending"`
	Permissions   string   `json:"permissions"`
	TimeoutExpiry string   `json:"communication_disabled_until"`
	GuildId       string   `json:"guild_id"`
	Roles         []string `json:"roles"`
	User          User     `json:"user"`
}

type Role struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	Color        int    `json:"color"`
	Hoist        bool   `json:"hoist"`
	Icon         string `json:"icon"`
	UnicodeEmoji bool   `json:"unicode_emoji"`
	Position     int    `json:"position"`
	Permissions  string `json:"permissions"`
	Managed      bool   `json:"managed"`
	Mentionable  bool   `json:"mentionable"`
	GuildId      string `json:"guild_id"`
}

type Emoji struct {
	Id            int64    `json:"id,string"`
	Name          string   `json:"name"`
	Roles         []string `json:"roles"`
	Managed       bool     `json:"managed"`
	Animated      bool     `json:"animated"`
	Available     bool     `json:"available"`
	RequireColons bool     `json:"require_colons"`
}

type Presence struct {
	Since    int64
	Status   Status
	AFK      bool
	Activity Activity
	OnMobile bool
}

type Status string

type Activity struct {
	Name string       `json:"name"`
	Type ActivityType `json:"type"`
	URL  string       `json:"url"`
}

type ActivityType int

type Runtime struct {
	SessionId        string `json:"session_id"`
	Shard            []int  `json:"shard"`
	ResumeGatewayURL string `json:"resume_gateway_url"`
	Guilds           []struct {
		Id string `json:"id"`
	} `json:"guilds"`
	Application struct {
		Id    string  `json:"id"`
		Flags float64 `json:"flags"`
	} `json:"application"`
	User Bot `json:"user"`
}

type Sharding struct {
	URL               string `json:"url"`
	Shards            int    `json:"shards"`
	SessionStartLimit struct {
		Total          int `json:"total"`
		Remaining      int `json:"remaining"`
		Reset          int `json:"reset_after"`
		MaxConcurrency int `json:"max_concurrency"`
	} `json:"session_start_limit"`
}

type Listeners struct {
	Ready             func(bot *Bot)
	MessageCreate     func(bot *Bot, message *Message)
	GuildCreate       func(bot *Bot, guild *Guild)
	GuildDelete       func(bot *Bot, guild *Guild)
	InteractionCreate func(bot *Bot, interaction *Interaction)
	VoiceStateUpdate  func(bot *Bot, voiceStates []VoiceState)
}

type Message struct {
	Id                 string                   `json:"id"`
	ChannelId          string                   `json:"channel_id"`
	GuildId            string                   `json:"guild_id"`
	Author             User                     `json:"author"`
	Content            string                   `json:"content"`
	Timestamp          string                   `json:"timestamp"`
	EditedTimestamp    string                   `json:"edited_timestamp"`
	TTS                bool                     `json:"tts"`
	MentionEveryone    bool                     `json:"mention_everyone"`
	Mentions           []User                   `json:"mentions"`
	RoleMentions       []string                 `json:"mention_roles"`
	ChannelMentions    []Channel                `json:"mention_channels"`
	Attachments        []map[string]interface{} `json:"attachments"`
	Embeds             []Embed                  `json:"embeds"`
	Reactions          []map[string]interface{} `json:"reactions"`
	Pinned             bool                     `json:"pinned"`
	WebhookId          string                   `json:"webhook_id"`
	Type               int                      `json:"types"`
	Activity           map[string]interface{}   `json:"activity"`
	Application        map[string]interface{}   `json:"application"`
	ApplicationId      string                   `json:"application_id"`
	MessageReference   map[string]interface{}   `json:"message_reference"`
	Flags              int                      `json:"flags"`
	ReferencedMessages map[string]interface{}   `json:"reference"`
	Interaction        map[string]interface{}   `json:"interaction"`
	Thread             map[string]interface{}   `json:"thread"`
	Components         []map[string]interface{} `json:"components"`
	Stickers           []map[string]interface{} `json:"sticker_items"`
}

type Interaction struct {
	Id            string `json:"id"`
	ApplicationId string `json:"application_id"`
	Type          int    `json:"type"`
	Data          struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Options  []Option `json:"options"`
		Resolved struct {
			Users       map[string]*User       `json:"users"`
			Members     map[string]*Member     `json:"members"`
			Roles       map[string]*Role       `json:"roles"`
			Channels    map[string]*Channel    `json:"channels"`
			Messages    map[string]*Message    `json:"messages"`
			Attachments map[string]*Attachment `json:"attachments"`
		}
	} `json:"data"`
	GuildId        string  `json:"guild_id"`
	ChannelId      string  `json:"channel_id"`
	User           User    `json:"user"`
	Token          string  `json:"token"`
	Version        int     `json:"version"`
	AppPermissions string  `json:"app_permissions"`
	Locale         string  `json:"locale"`
	GuildLocale    string  `json:"guild_locale"`
	Message        Message `json:"message"`
	Channel        Channel `json:"-"`
	Guild          Guild   `json:"-"`
	Author         Member  `json:"member"`
}

type EmbedImage struct {
	URL      string `json:"url"`
	Height   int    `json:"height"`
	Width    int    `json:"width"`
	ProxyURL string `json:"proxy_url"`
}

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"Value"`
	Inline bool   `json:"inline"`
}

type EmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url"`
}

type EmbedAuthor struct {
	Name    string `json:"name"`
	IconURL string `json:"icon_url"`
}

type Embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	URL         string       `json:"url"`
	Timestamp   string       `json:"timestamp"`
	Color       int          `json:"color"`
	Footer      EmbedFooter  `json:"footer"`
	Author      EmbedAuthor  `json:"author"`
	Image       EmbedImage   `json:"image"`
	Thumbnail   EmbedImage   `json:"thumbnail"`
	Fields      []EmbedField `json:"fields"`
}

type Command struct {
	Name             string
	Description      string
	Options          []Option
	DMPermission     bool
	Permissions      []Permission
	GuildId          int
	Subcommands      []SubCommand
	SubcommandGroups []SubcommandGroup
}

type SubCommand struct {
	Name        string
	Description string
	Options     []Option
}

type SubcommandGroup struct {
	Name        string
	Description string
	Subcommands []SubCommand
}

type Option struct {
	Name         string `json:"name"`
	Type         int    `json:"type"`
	Description  string
	Required     bool
	MinLength    int
	MaxLength    int
	MinValue     int
	MaxValue     int
	MaxValueNum  float64
	MinValueNum  float64
	AutoComplete bool
	ChannelTypes []int
	Choices      []Choice
	Value        interface{} `json:"Value"`
	Focused      bool        `json:"focused"`
}

type Choice struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type Permission int

type ComponentData struct {
	ComponentType int      `json:"component_type"`
	CustomId      string   `json:"custom_id"`
	Values        []string `json:"values"`
	Components    []Row    `json:"components"`
}

type Component struct {
	CustomId string   `json:"custom_id"`
	Type     int      `json:"type"`
	Value    string   `json:"value"`
	Values   []string `json:"values"`
}

type Row struct {
	Components []Component
}

type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Description string `json:"description"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	ProxyURL    string `json:"proxy_url"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Ephemeral   bool   `json:"ephemeral"`
}

type Response struct {
	Content        string
	Embeds         []Embed
	TTS            bool
	Ephemeral      bool
	SuppressEmbeds bool
	Files          []File
}

type File struct {
	Name        string
	Description string
	Content     []byte
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err *Error) Error() string {
	return "[" + strconv.Itoa(err.Code) + "]:" + err.Message
}

type Voice struct {
	GuildId   string
	ChannelId string
	Send      chan []byte
	ready     chan bool
	state     chan *VoiceState
	server    chan *VoiceServerUpdate
	mute      bool
	deaf      bool
	conn      *socket.Conn
	two       struct {
		SSRC              uint32        `json:"ssrc"`
		Port              int           `json:"port"`
		Modes             []string      `json:"modes"`
		HeartbeatInterval time.Duration `json:"heartbeat_interval"`
		IP                string        `json:"ip"`
	}
	four struct {
		SecretKey [32]byte `json:"secret_key"`
		Mode      string   `json:"mode"`
	}
	udp *net.UDPConn
	err chan error
}

type VoiceState struct {
	GuildID                 string    `json:"guild_id"`
	ChannelID               string    `json:"channel_id"`
	UserID                  string    `json:"user_id"`
	Member                  Member    `json:"member"`
	SessionID               string    `json:"session_id"`
	Deaf                    bool      `json:"deaf"`
	Mute                    bool      `json:"mute"`
	SelfDeaf                bool      `json:"self_deaf"`
	SelfMute                bool      `json:"self_mute"`
	SelfStream              bool      `json:"self_stream"`
	SelfVideo               bool      `json:"self_video"`
	Suppress                bool      `json:"suppress"`
	RequestToSpeakTimestamp time.Time `json:"request_to_speak_timestamp"`
}

type VoiceServerUpdate struct {
	Token    string `json:"token"`
	GuildID  string `json:"guild_id"`
	Endpoint string `json:"endpoint"`
}
