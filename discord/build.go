package discord

import (
	"errors"
	"strings"
)

func (opt *Option) build() map[string]interface{} {
	body := map[string]interface{}{
		"name":        opt.Name,
		"type":        opt.Type,
		"required":    opt.Required,
		"description": opt.Description,
	}

	switch opt.Type {
	case StringOption:
		body["min_length"] = opt.MinLength
		body["max_length"] = opt.MaxLength
	case IntOption:
		body["min_value"] = opt.MinValue
		body["max_value"] = opt.MaxValue
	case NumOption:
		body["min_value"] = opt.MinValueNum
		body["max_value"] = opt.MaxValueNum
	case ChannelOption:
		body["channel_types"] = make([]int, len(opt.ChannelTypes))
		body["channel_types"] = append(body["channel_types"].([]int), opt.ChannelTypes...)
	}

	if len(opt.Choices) > 0 {
		body["choices"] = opt.Choices
	}
	if opt.AutoComplete {
		body["auto_complete"] = true
	}

	return body
}

func (sub *SubCommand) build() map[string]interface{} {
	body := map[string]interface{}{
		"name":        sub.Name,
		"type":        1,
		"description": sub.Description,
	}

	bodies := make([]map[string]interface{}, len(sub.Options))
	for idx := range sub.Options {
		bodies[idx] = sub.Options[idx].build()
	}

	body["options"] = bodies

	return body
}

func (subGroup *SubcommandGroup) build() map[string]interface{} {
	body := map[string]interface{}{
		"name":        subGroup.Name,
		"type":        2,
		"description": subGroup.Description,
	}

	bodies := make([]map[string]interface{}, len(subGroup.Subcommands))
	for idx := range subGroup.Subcommands {
		bodies[idx] = subGroup.Subcommands[idx].build()
	}

	body["options"] = bodies

	return body
}

func (res *Response) build() (map[string]interface{}, error) {
	flag := 0
	body := make(map[string]interface{})

	if res.Content != "" {
		body["content"] = res.Content
	}
	if len(res.Embeds) > 10 {
		res.Embeds = res.Embeds[:10]
	}
	if res.TTS {
		body["tts"] = true
	}
	if res.Ephemeral {
		flag |= 1 << 6
	}
	if res.SuppressEmbeds {
		flag |= 1 << 2
	}
	if res.Ephemeral || res.SuppressEmbeds {
		body["flags"] = flag
	}

	embeds := make([]map[string]interface{}, len(res.Embeds))
	for idx := range res.Embeds {
		embed := map[string]interface{}{
			"color":     res.Embeds[idx].Color,
			"timestamp": res.Embeds[idx].Timestamp,
			"author": map[string]interface{}{
				"name":     res.Embeds[idx].Author.Name,
				"icon_url": res.Embeds[idx].Author.IconURL,
			},
			"footer": map[string]interface{}{
				"text":     res.Embeds[idx].Footer.Text,
				"icon_url": res.Embeds[idx].Footer.IconURL,
			},
			"image": map[string]interface{}{
				"url": res.Embeds[idx].Image.URL,
			},
			"thumbnail": map[string]interface{}{
				"url": res.Embeds[idx].Thumbnail.URL,
			},
		}

		if len([]rune(res.Embeds[idx].Title)) > 256 {
			return nil, errors.New("embed is longer than 256 characters")
		}
		embed["title"] = res.Embeds[idx].Title

		if len([]rune(res.Embeds[idx].Description)) > 4096 {
			return nil, errors.New("description is longer than 4096 characters")
		}
		embed["description"] = res.Embeds[idx].Description

		if res.Embeds[idx].URL != "" {
			if !strings.HasPrefix(res.Embeds[idx].URL, "http") {
				return nil, errors.New("url didn't start with http/https")
			}
			embed["url"] = res.Embeds[idx].URL
		}

		fields := make([]map[string]interface{}, len(res.Embeds[idx].Fields))
		for idx, field := range res.Embeds[idx].Fields {
			fields[idx] = map[string]interface{}{
				"name":   field.Name,
				"value":  field.Value,
				"inline": field.Inline,
			}
		}
		embeds[idx] = embed
	}
	body["embeds"] = embeds

	attachments := make([]map[string]interface{}, len(res.Files))
	for idx := range res.Files {
		attachments[idx] = map[string]interface{}{
			"id":          idx,
			"filename":    res.Files[idx].Name,
			"description": res.Files[idx].Description,
		}
	}
	body["attachments"] = attachments

	return body, nil
}
