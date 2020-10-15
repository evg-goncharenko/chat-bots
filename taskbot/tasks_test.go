package main

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"sync/atomic"
	"fmt"
	"gopkg.in/telegram-bot-api.v4"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func init() {
/*
	Upd global var for testing
	we use patched version of gopkg.in/telegram-bot-api.v4 ( WebhookURL const -> var)
*/
	WebhookURL = "http://127.0.0.1:8081"
	BotToken = "_golangcourse_test"
}

var (
	client = &http.Client{Timeout: time.Second}
)

// TDS is Telegram Dummy Server
type TDS struct {
	*sync.Mutex
	Answers map[int]string
}

func NewTDS() *TDS {
	return &TDS{
		Mutex:   &sync.Mutex{},
		Answers: make(map[int]string),
	}
}

func (srv *TDS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("/getMe", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true,"result":{"id":` +
			strconv.Itoa(BotChatID) +
			`,"is_bot":true,"first_name":"game_test_bot","username":"game_test_bot"}}`))
	})
	mux.HandleFunc("/setWebhook", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true,"result":true,"description":"Webhook was set"}`))
	})
	mux.HandleFunc("/sendMessage", func(w http.ResponseWriter, r *http.Request) {
		chatID, _ := strconv.Atoi(r.FormValue("chat_id"))
		text := r.FormValue("text")
		srv.Lock()
		srv.Answers[chatID] = text
		srv.Unlock()
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		panic(fmt.Errorf("unknown command %s", r.URL.Path))
	})

	handler := http.StripPrefix("/bot"+BotToken, mux)
	handler.ServeHTTP(w, r)
}

const (
	Ivanov     int = 256
	Petrov     int = 512
	Alexandrov int = 1024
	BotChatID      = 100500
)

var (
	users = map[int]*tgbotapi.User{
		Ivanov: &tgbotapi.User{
			ID:           Ivanov,
			FirstName:    "Ivan",
			LastName:     "Ivanov",
			UserName:     "ivanov",
			LanguageCode: "ru",
			IsBot:        false,
		},
		Petrov: &tgbotapi.User{
			ID:           Petrov,
			FirstName:    "Petr",
			LastName:     "Pertov",
			UserName:     "ppetrov",
			LanguageCode: "ru",
			IsBot:        false,
		},
		Alexandrov: &tgbotapi.User{
			ID:           Alexandrov,
			FirstName:    "Alex",
			LastName:     "Alexandrov",
			UserName:     "aalexandrov",
			LanguageCode: "ru",
			IsBot:        false,
		},
	}

	updID uint64
	msgID uint64
)

func SendMsgToBot(userID int, text string) error {
	// reqText := `{
	// 	"update_id":175894614,
	// 	"message":{
	// 		"message_id":29,
	// 		"from":{"id":133250764,"is_bot":false,"first_name":"Vasily Romanov","username":"rvasily","language_code":"ru"},
	// 		"chat":{"id":133250764,"first_name":"Vasily Romanov","username":"rvasily","type":"private"},
	// 		"date":1512168732,
	// 		"text":"THIS SEND FROM USER"
	// 	}
	// }`

	atomic.AddUint64(&updID, 1)
	myUpdID := atomic.LoadUint64(&updID)

	// better have it per user, but lazy now
	atomic.AddUint64(&msgID, 1)
	myMsgID := atomic.LoadUint64(&msgID)

	user, ok := users[userID]
	if !ok {
		return fmt.Errorf("no user for %d", userID)
	}

	upd := &tgbotapi.Update{
		UpdateID: int(myUpdID),
		Message: &tgbotapi.Message{
			MessageID: int(myMsgID),
			From:      user,
			Chat: &tgbotapi.Chat{
				ID:        int64(user.ID),
				FirstName: user.FirstName,
				UserName:  user.UserName,
				Type:      "private",
			},
			Text: text,
			Date: int(time.Now().Unix()),
		},
	}
	reqData, _ := json.Marshal(upd)

	reqBody := bytes.NewBuffer(reqData)
	req, _ := http.NewRequest(http.MethodPost, WebhookURL, reqBody)
	_, err := client.Do(req)
	return err
}

type testCase struct {
	user    int
	command string
	answers map[int]string
}

func TestTasks(t *testing.T) {

	tds := NewTDS()
	ts := httptest.NewServer(tds)
	tgbotapi.APIEndpoint = ts.URL + "/bot%s/%s"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := startTaskBot(ctx)
		if err != nil {
			t.Fatalf("startTaskBot error: %s", err)
		}
	}()

	// give server time to start
	time.Sleep(10 * time.Millisecond)

	cases := []testCase{
		{
			// /tasks - displays a list of all active tasks
			Ivanov,
			"/tasks",
			map[int]string{
				Ivanov: "Нет задач",
			},
		},
		{
			// command /new - creates a new task, everything after /new - goes to the task name
			Ivanov,
			"/new написать бота",
			map[int]string{
				Ivanov: `Задача "написать бота" создана, id=1`,
			},
		},
		{
			Ivanov,
			"/tasks",
			map[int]string{
				Ivanov: `1. написать бота by @ivanov
/assign_1`,
			},
		},
		{
			// /assign_* - assigns the task to itself
			Alexandrov,
			"/assign_1",
			map[int]string{
				Alexandrov: `Задача "написать бота" назначена на вас`,
				Ivanov:     `Задача "написать бота" назначена на @aalexandrov`,
			},
		},
		{
		/*
			if the task was assigned to someone, they get a notification about it
			in this case, it was assigned to Alexandrov, so a notification is sent to Him
		*/
			Petrov,
			"/assign_1",
			map[int]string{
				Petrov:     `Задача "написать бота" назначена на вас`,
				Alexandrov: `Задача "написать бота" назначена на @ppetrov`,
			},
		},
		{
			// if the task is also assigned to me, "to me" is displayed"
			Petrov,
			"/tasks",
			map[int]string{
				Petrov: `1. написать бота by @ivanov
assignee: я
/unassign_1 /resolve_1`,
			},
		},
		{
			// if the task is assigned and not on me, the performer's username is shown
			Ivanov,
			"/tasks",
			map[int]string{
				Ivanov: `1. написать бота by @ivanov
assignee: @ppetrov`,
			},
		},

		{
			
		/*
			/unassign_ - removes the task from itself
			you can't remove an issue that isn't on you
		*/
			Alexandrov,
			"/unassign_1",
			map[int]string{
				Alexandrov: `Задача не на вас`,
			},
		},

		{
		/*
			/unassign_ - removes the task from itself
			a notification is sent to the author that the task was left without a performer
		*/
			Petrov,
			"/unassign_1",
			map[int]string{
				Petrov: `Принято`,
				Ivanov: `Задача "написать бота" осталась без исполнителя`,
			},
		},

		{
		/*
			repeat
			if the task was assigned to someone, the author receives a notification about it
		*/
			Petrov,
			"/assign_1",
			map[int]string{
				Petrov: `Задача "написать бота" назначена на вас`,
				Ivanov: `Задача "написать бота" назначена на @ppetrov`,
			},
		},
		{
		/*
			/resolve_* completes the task and deletes it from the storage
			the author is notified about this
		*/
			Petrov,
			"/resolve_1",
			map[int]string{
				Petrov: `Задача "написать бота" выполнена`,
				Ivanov: `Задача "написать бота" выполнена @ppetrov`,
			},
		},

		{
			Petrov,
			"/tasks",
			map[int]string{
				Petrov: `Нет задач`,
			},
		},

		{
			// note that id=2 is an auto-increment
			Petrov,
			"/new сделать ДЗ по курсу",
			map[int]string{
				Petrov: `Задача "сделать ДЗ по курсу" создана, id=2`,
			},
		},
		{
			// note that id=3 is an auto-increment
			Ivanov,
			"/new прийти на хакатон",
			map[int]string{
				Ivanov: `Задача "прийти на хакатон" создана, id=3`,
			},
		},
		{
			Petrov,
			"/tasks",
			map[int]string{
				Petrov: `2. сделать ДЗ по курсу by @ppetrov
/assign_2

3. прийти на хакатон by @ivanov
/assign_3`,
			},
		},
		{
		/*
			repeat
			if the task was assigned to someone, the author receives a notification about it
			if he is the author of the task, he does not receive an additional notification that it is assigned to someone
			
		*/
			Petrov,
			"/assign_2",
			map[int]string{
				Petrov: `Задача "сделать ДЗ по курсу" назначена на вас`,
			},
		},
		{
			Petrov,
			"/tasks",
			map[int]string{
				Petrov: `2. сделать ДЗ по курсу by @ppetrov
assignee: я
/unassign_2 /resolve_2

3. прийти на хакатон by @ivanov
/assign_3`,
			},
		},
		{
		/*
			/my shows the tasks that are assigned to me
			at this time there is no label assegnee
		*/
			Petrov,
			"/my",
			map[int]string{
				Petrov: `2. сделать ДЗ по курсу by @ppetrov
/unassign_2 /resolve_2`,
			},
		},
		{
		/*
			/owner - shows the tasks that I created
			at this time there is no label assegnee
		*/
			Ivanov,
			"/owner",
			map[int]string{
				Ivanov: `3. прийти на хакатон by @ivanov
/assign_3`,
			},
		},
	}

	for idx, item := range cases {

		tds.Lock()
		tds.Answers = make(map[int]string)
		tds.Unlock()

		caseName := fmt.Sprintf("[case%d, %d: %s]", idx, item.user, item.command)
		err := SendMsgToBot(item.user, item.command)
		if err != nil {
			t.Fatalf("%s SendMsgToBot error: %s", caseName, err)
		}
		// give TDS time to process request
		time.Sleep(10 * time.Millisecond)

		tds.Lock()
		result := reflect.DeepEqual(tds.Answers, item.answers)
		if !result {
			t.Fatalf("%s bad results:\n\tWant: %v\n\tHave: %v", caseName, item.answers, tds.Answers)
		}
		tds.Unlock()

	}

}
