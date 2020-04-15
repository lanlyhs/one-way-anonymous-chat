package app

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type clientMessageRelation struct {
	msgIds []int
	client *websocket.Conn
}

var cache = make(map[string]clientMessageRelation)

type botClient struct {
	api    *tgbotapi.BotAPI
	chatID int64
}

func newBotClient(token string, chatID int64) *botClient {

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logrus.Fatal("input chatID is error", err)
	}

	bot.Debug = false

	logrus.Infof("Authorized on account %s", bot.Self.UserName)

	bc := &botClient{bot, chatID}
	go bc.recvMsg()

	return bc
}

func (b *botClient) recvMsg() {

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := b.api.GetUpdatesChan(u)
	if err != nil {
		logrus.Info("GetUpdatesChan:", err)
		return
	}

	contains := func(msgIds []int, mId int) bool {
		for _, id := range msgIds {
			if id == mId {
				return true
			}
		}
		return false
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.Chat.ID != b.chatID {
			continue
		}

		replyToMessage := update.Message.ReplyToMessage
		if replyToMessage == nil {
			continue
		}

		logrus.Infof("[%s] %s", update.Message.From.UserName, update.Message.Text)

		for _, v := range cache {
			if contains(v.msgIds, replyToMessage.MessageID) {
				err := v.client.WriteMessage(websocket.TextMessage, []byte(update.Message.Text))
				if err != nil {
					logrus.Error("write:", err)
					break
				}
			}
		}
	}
}

func (b *botClient) sendMsg(msg []byte, name string, conn *websocket.Conn) {

	nm := tgbotapi.NewMessage(b.chatID, name+"\n\n"+string(msg))
	m, err := b.api.Send(nm)
	if err != nil {
		logrus.Error("sendMsg:", err)
		return
	}

	if v, ok := cache[name]; ok {
		v.msgIds = append(v.msgIds, m.MessageID)
		cache[name] = clientMessageRelation{
			msgIds: v.msgIds,
			client: v.client,
		}
		return
	}

	cache[name] = clientMessageRelation{
		msgIds: []int{m.MessageID},
		client: conn,
	}
}

func cleanCache(name string) {
	cmr := cache[name]

	// Clean Websocket Conn
	err := cmr.client.Close()
	if err != nil {
		logrus.Error("Clean Cache Websocket Conn Close:", err)
	}

	// Clean User and MessgeIDs Realtion
	cmr.msgIds = nil

	delete(cache, name)
}
