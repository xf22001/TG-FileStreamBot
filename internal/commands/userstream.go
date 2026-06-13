package commands

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/userstream"
	"EverythingSuckz/fsb/internal/utils"
	"fmt"
	"strings"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/storage"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/tg"
)

func (m *command) LoadUserStream(dispatcher dispatcher.Dispatcher) {
	log := m.log.Named("userstream")
	defer log.Sugar().Info("Loaded")
	dispatcher.AddHandler(handlers.NewMessage(nil, linkToUserStream))
}

func linkToUserStream(ctx *ext.Context, u *ext.Update) error {
	text := effectiveText(u)
	if !strings.HasPrefix(text, "https://t.me/") && !strings.HasPrefix(text, "https://telegram.me/") {
		return nil
	}
	if strings.Contains(text, " ") {
		return nil
	}
	return replyUserStreamInfo(ctx, u, text, true)
}

func effectiveText(u *ext.Update) string {
	if u.EffectiveMessage == nil || u.EffectiveMessage.Message == nil {
		return ""
	}
	return strings.TrimSpace(u.EffectiveMessage.Message.Message)
}

func replyUserStreamInfo(ctx *ext.Context, u *ext.Update, rawLink string, verbose bool) error {
	chatID := u.EffectiveChat().GetID()
	peerChatID := ctx.PeerStorage.GetPeerById(chatID)
	if peerChatID.Type != int(storage.TypeUser) {
		return dispatcher.EndGroups
	}
	if len(config.ValueOf.AllowedUsers) != 0 && !utils.Contains(config.ValueOf.AllowedUsers, chatID) {
		ctx.Reply(u, ext.ReplyTextString("You are not allowed to use this bot."), nil)
		return dispatcher.EndGroups
	}
	if !userstream.Ready() {
		ctx.Reply(u, ext.ReplyTextString("USER_SESSION is not configured; user identity link streaming is unavailable."), nil)
		return dispatcher.EndGroups
	}

	infos, err := userstream.ResolveInfos(ctx, rawLink, config.ValueOf.Host)
	if err != nil {
		ctx.Reply(u, ext.ReplyTextString(fmt.Sprintf("Error - %s", err.Error())), nil)
		return dispatcher.EndGroups
	}

	for _, info := range infos {
		message := formatInfoMessage(info, verbose)
		row := tg.KeyboardButtonRow{Buttons: []tg.KeyboardButtonClass{
			&tg.KeyboardButtonURL{Text: "Download", URL: info.DownloadURL},
		}}
		if strings.Contains(info.MimeType, "video") || strings.Contains(info.MimeType, "audio") || strings.Contains(info.MimeType, "pdf") || strings.HasPrefix(info.MimeType, "image/") {
			row.Buttons = append(row.Buttons, &tg.KeyboardButtonURL{Text: "Stream", URL: info.StreamURL})
		}
		markup := &tg.ReplyInlineMarkup{Rows: []tg.KeyboardButtonRow{row}}

		_, err = ctx.Reply(u, ext.ReplyTextStyledTextArray([]styling.StyledTextOption{
			styling.Plain(message + "\n\n"),
			styling.Code(info.StreamURL),
			styling.Plain("\n"),
			styling.Code(info.DownloadURL),
		}), &ext.ReplyOpts{
			Markup:           markup,
			NoWebpage:        false,
			ReplyToMessageId: u.EffectiveMessage.ID,
		})
		if err != nil {
			utils.Logger.Sugar().Error(err)
		}
	}
	return dispatcher.EndGroups
}

func formatInfoMessage(info *userstream.Info, verbose bool) string {
	chatName := info.ChatTitle
	if chatName == "" {
		chatName = info.ChatUser
	}
	if chatName == "" {
		chatName = fmt.Sprintf("%d", info.ChatID)
	}
	if !verbose {
		return fmt.Sprintf("%s\n%s", info.StreamURL, summaryLine(info))
	}
	caption := info.Caption
	if len(caption) > 300 {
		caption = caption[:300] + "..."
	}
	return fmt.Sprintf(
		"Group/Channel\n├─ id: %d\n├─ name: %s\n└─ username: %s\nMessage\n├─ id: %d\n├─ type: %s\n├─ file_name: %s\n├─ file_size: %s\n├─ mime_type: %s\n└─ caption: %s",
		info.ChatID,
		chatName,
		info.ChatUser,
		info.MessageID,
		info.MediaType,
		info.FileName,
		formatBytes(info.FileSize),
		info.MimeType,
		caption,
	)
}

func summaryLine(info *userstream.Info) string {
	return fmt.Sprintf("%s | %s | %s", info.FileName, formatBytes(info.FileSize), info.MimeType)
}

func formatBytes(size int64) string {
	if size <= 0 {
		return "unknown"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d%s", size, units[unit])
	}
	return fmt.Sprintf("%.2f%s", value, units[unit])
}
