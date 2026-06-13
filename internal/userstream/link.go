package userstream

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/storage"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

var (
	clientMu sync.RWMutex
	client   *gotgproto.Client
	log      *zap.Logger

	storeMu sync.RWMutex
	store   = make(map[string]*Ref)
)

type Link struct {
	Raw       string
	GroupID   string
	ChannelID int64
	MessageID int
	TopicID   int
	CommentID int
	Private   bool
}

type Ref struct {
	Token     string
	Link      Link
	File      *types.File
	CreatedAt time.Time
}

type Info struct {
	Link        Link
	ChatTitle   string
	ChatUser    string
	ChatID      int64
	MessageID   int
	Caption     string
	MediaType   string
	FileName    string
	FileSize    int64
	MimeType    string
	StreamURL   string
	PreviewURL  string
	DownloadURL string
}

func SetClient(c *gotgproto.Client, l *zap.Logger) {
	clientMu.Lock()
	defer clientMu.Unlock()
	client = c
	log = l.Named("UserStream")
}

func Ready() bool {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return client != nil
}

func Client() *gotgproto.Client {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return client
}

func ParseLink(raw string) (Link, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Link{}, errors.New("empty link")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Link{}, err
	}
	host := strings.ToLower(u.Hostname())
	if host != "t.me" && host != "telegram.me" {
		return Link{}, errors.New("not a Telegram message link")
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(strings.Trim(u.Path, "/"), "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) < 2 {
		return Link{}, errors.New("message link must include chat and message id")
	}

	link := Link{Raw: raw}
	if parts[0] == "c" {
		if len(parts) < 3 {
			return Link{}, errors.New("private link must be /c/<channel>/<message>")
		}
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return Link{}, err
		}
		link.ChannelID = id
		link.GroupID = fmt.Sprintf("-100%d", id)
		link.Private = true
		if len(parts) >= 4 {
			link.TopicID, _ = strconv.Atoi(parts[2])
			link.MessageID, err = strconv.Atoi(parts[3])
		} else {
			link.MessageID, err = strconv.Atoi(parts[2])
		}
		if err != nil {
			return Link{}, err
		}
	} else {
		link.GroupID = parts[0]
		var err error
		if len(parts) >= 3 {
			link.TopicID, _ = strconv.Atoi(parts[1])
			link.MessageID, err = strconv.Atoi(parts[2])
		} else {
			link.MessageID, err = strconv.Atoi(parts[1])
		}
		if err != nil {
			return Link{}, err
		}
	}
	if comment := u.Query().Get("comment"); comment != "" {
		link.CommentID, _ = strconv.Atoi(comment)
	}
	if link.CommentID != 0 {
		link.MessageID = link.CommentID
	}
	if link.MessageID <= 0 {
		return Link{}, errors.New("invalid message id")
	}
	return link, nil
}

func ResolveInfos(ctx context.Context, rawLink, host string) ([]*Info, error) {
	c := Client()
	if c == nil {
		return nil, errors.New("USER_SESSION is not configured")
	}
	link, err := ParseLink(rawLink)
	if err != nil {
		return nil, err
	}
	msgs, chat, err := getMessages(ctx, c, link)
	if err != nil {
		return nil, err
	}

	// Telegram media groups often have the caption only on the first item.
	// We find the first non-empty caption and apply it to all items in the group.
	commonCaption := ""
	for _, m := range msgs {
		if m.Message != "" {
			commonCaption = m.Message
			break
		}
	}

	var infos []*Info
	for _, msg := range msgs {
		file, err := utils.FileFromMedia(msg.Media)
		if err != nil {
			continue
		}
		token, _, err := Save(link, file)
		if err != nil {
			continue
		}
		streamURL := strings.TrimRight(host, "/") + "/u/stream/" + token
		caption := msg.Message
		if caption == "" {
			caption = commonCaption
		}
		info := &Info{
			Link:        link,
			ChatID:      chat.GetID(),
			MessageID:   msg.ID,
			Caption:     caption,
			FileName:    file.FileName,
			FileSize:    file.FileSize,
			MimeType:    file.MimeType,
			StreamURL:   streamURL,
			DownloadURL: streamURL + "?d=true",
		}
		if channel, ok := chat.(*tg.Channel); ok {
			info.ChatTitle = channel.Title
			info.ChatUser = channel.Username
		}
		if file.FileSize == 0 || strings.HasPrefix(file.MimeType, "image/") {
			info.MediaType = "photo"
			info.PreviewURL = streamURL
		} else if strings.HasPrefix(file.MimeType, "video/") {
			info.MediaType = "video"
			info.PreviewURL = streamURL
		} else if strings.HasPrefix(file.MimeType, "audio/") {
			info.MediaType = "audio"
		} else {
			info.MediaType = "document"
		}
		infos = append(infos, info)
	}
	if len(infos) == 0 {
		return nil, errors.New("no media found in the message(s)")
	}
	return infos, nil
}

func Get(token string) (*Ref, bool) {
	storeMu.RLock()
	defer storeMu.RUnlock()
	ref, ok := store[token]
	return ref, ok
}

func Save(link Link, file *types.File) (string, *Ref, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	token := hex.EncodeToString(buf)
	ref := &Ref{Token: token, Link: link, File: file, CreatedAt: time.Now()}
	storeMu.Lock()
	store[token] = ref
	storeMu.Unlock()
	return token, ref, nil
}

func getMessages(ctx context.Context, c *gotgproto.Client, link Link) ([]*tg.Message, tg.ChatClass, error) {
	channel, chat, err := resolveChannel(ctx, c, link)
	if err != nil {
		return nil, nil, err
	}
	res, err := c.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: channel,
		ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: link.MessageID}},
	})
	if err != nil {
		return nil, nil, err
	}
	messages, ok := res.(*tg.MessagesChannelMessages)
	if !ok || len(messages.Messages) == 0 {
		return nil, nil, errors.New("message not found")
	}
	baseMsg, ok := messages.Messages[0].(*tg.Message)
	if !ok {
		return nil, nil, errors.New("message is empty or inaccessible")
	}

	if baseMsg.GroupedID == 0 {
		return []*tg.Message{baseMsg}, chat, nil
	}

	// If it's a grouped message, fetch surrounding messages to find siblings
	searchIDs := make([]tg.InputMessageClass, 0)
	for i := link.MessageID - 10; i <= link.MessageID + 10; i++ {
		searchIDs = append(searchIDs, &tg.InputMessageID{ID: i})
	}

	resGroup, err := c.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: channel,
		ID:      searchIDs,
	})
	if err != nil {
		return []*tg.Message{baseMsg}, chat, nil // Fallback to single message
	}

	groupMessages, ok := resGroup.(*tg.MessagesChannelMessages)
	if !ok {
		return []*tg.Message{baseMsg}, chat, nil
	}

	var result []*tg.Message
	for _, m := range groupMessages.Messages {
		if msg, ok := m.(*tg.Message); ok {
			if msg.GroupedID == baseMsg.GroupedID {
				result = append(result, msg)
			}
		}
	}

	// Sort by ID to maintain order
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].ID > result[j].ID {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if len(result) == 0 {
		return []*tg.Message{baseMsg}, chat, nil
	}

	return result, chat, nil
}

func resolveChannel(ctx context.Context, c *gotgproto.Client, link Link) (*tg.InputChannel, tg.ChatClass, error) {
	if !link.Private {
		resolved, err := c.API().ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: link.GroupID})
		if err != nil {
			return nil, nil, err
		}
		peer, ok := resolved.Peer.(*tg.PeerChannel)
		if !ok {
			return nil, nil, errors.New("link does not point to a channel/supergroup message")
		}
		for _, chat := range resolved.Chats {
			if channel, ok := chat.(*tg.Channel); ok && channel.ID == peer.ChannelID {
				c.PeerStorage.AddPeer(channel.ID, channel.AccessHash, storage.TypeChannel, channel.Username)
				return channel.AsInput(), chat, nil
			}
		}
		return nil, nil, errors.New("resolved channel info not found")
	}

	ids := []int64{link.ChannelID, -1000000000000 + link.ChannelID}
	for _, id := range ids {
		inputPeer := c.PeerStorage.GetInputPeerById(id)
		if peer, ok := inputPeer.(*tg.InputPeerChannel); ok && peer.AccessHash != 0 {
			input := &tg.InputChannel{ChannelID: peer.ChannelID, AccessHash: peer.AccessHash}
			chats, err := c.API().ChannelsGetChannels(ctx, []tg.InputChannelClass{input})
			if err == nil && len(chats.GetChats()) > 0 {
				return input, chats.GetChats()[0], nil
			}
			return input, &tg.Channel{ID: peer.ChannelID, AccessHash: peer.AccessHash}, nil
		}
	}

	input := &tg.InputChannel{ChannelID: link.ChannelID}
	chats, err := c.API().ChannelsGetChannels(ctx, []tg.InputChannelClass{input})
	if err != nil {
		if log != nil {
			log.Warn("failed to resolve private channel by id", zap.Int64("channelID", link.ChannelID), zap.Error(err))
		}
		return nil, nil, errors.New("private /c link requires the userbot to have this channel in peer cache; open/sync the chat first")
	}
	if len(chats.GetChats()) == 0 {
		return nil, nil, errors.New("private channel not found")
	}
	channel, ok := chats.GetChats()[0].(*tg.Channel)
	if !ok {
		return nil, nil, errors.New("unexpected chat type")
	}
	c.PeerStorage.AddPeer(channel.ID, channel.AccessHash, storage.TypeChannel, channel.Username)
	return channel.AsInput(), channel, nil
}
