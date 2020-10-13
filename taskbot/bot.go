package main

import (
"context"
"fmt"
tgbotapi "gopkg.in/telegram-bot-api.v4"
"log"
"net/http"
"os"
"strconv"
"strings"
)

type Task struct {
	Name     string
	Author   string
	Assignee string
}

var (
	BotToken   = "1224280674:AAHLx9qQLrfEiPK5Ru87kXKIGxu71X1usCs"
	WebhookURL = "https://b567e3ab.ngrok.io"

	AutoIncrement int = 1

	Users          map[string]int64 = make(map[string]int64)
	TaskManagement map[int]Task     = make(map[int]Task)
)
// output of all current user tasks: /tasks
func OutputCurrentTasks(pers string) string {
	var result string

	size := len(TaskManagement)
	if size == 0 {
		return "Нет задач"
	}

	for i := 1; i <= AutoIncrement; i++ {
		j, proof := TaskManagement[i]
		if proof != false {
			if len(result) > 10 {
				result += "\n\n"
			}
			result = result + strconv.Itoa(i) + ". " + j.Name + " by @" + j.Author
			switch j.Assignee {
			case pers:
				result = result + "\nassignee: я" + "\n/unassign_" + strconv.Itoa(i) + " /resolve_" + strconv.Itoa(i)
			case "":
				result = result + "\n/assign_" + strconv.Itoa(i)
			default:
				result = result + "\nassignee: @" + j.Assignee
			}
		}
	}
	return result
}

// creating a new task: /new XXX YYY ZZZ
func CreatingNewTask(taskName string, author string) string { 
	size := len(TaskManagement)
	if size != 0 {
		for i, j := range TaskManagement {
			if taskName == j.Name {
				return "Задача уже существует, id = " + strconv.Itoa(i)
			}
		}
	}
	var NewTypeTask Task
	NewTypeTask.Name = taskName
	NewTypeTask.Author = author
	TaskManagement[AutoIncrement] = NewTypeTask
	AutoIncrement++
	return "Задача \"" + taskName + "\" создана, id=" + strconv.Itoa(AutoIncrement-1)
}

// switching the task performer to the user: /assign_$ID
func SwitchingTaskPerformer(id int, person string) (string, string) { 
	var res1, res2 string
	tsk, err := TaskManagement[id]
	if err == false {
		res1 = "Попытка действия с несуществующей задачей"
		return res1, res2
	}
	tsk.Assignee = person
	var newT Task
	newT.Assignee = person
	newT.Author = tsk.Author
	newT.Name = tsk.Name
	TaskManagement[id] = newT
	res1 = "Задача \"" + tsk.Name + "\" назначена на вас"
	if tsk.Author != person {
		res2 = "Задача \"" + tsk.Name + "\" назначена на @" + tsk.Assignee
	}
	return res1, res2
}

// removing a task from the current performer: /unassign_$ID
func RemovingTaskPerformer(id int, person string) (string, string) { 
	var res1, res2 string
	tsk, err := TaskManagement[id]
	if err == false {
		res1 = "Попытка действия с несуществующей задачей"
		return res1, res2
	}
	if tsk.Assignee != person {
		res1 = "Задача не на вас"
		return res1, res2
	}
	res1 = "Принято"
	res2 = "Задача \"" + tsk.Name + "\" осталась без исполнителя"

	var newT Task
	newT.Assignee = ""
	newT.Author = tsk.Author
	newT.Name = tsk.Name
	TaskManagement[id] = newT

	return res1, res2
}

// completing a task and removing it from the list: /resolve_$ID
func ExecutionAndDeletion(id int, person string) string { 
	tsk, err := TaskManagement[id]
	if err == false {
		return "Несуществующая задача"
	}
	taskName := tsk.Name
	taskAssigne := tsk.Assignee
	taskAuthor := tsk.Author
	delete(TaskManagement, id)
	if taskAuthor == person {
		return "Задача \"" + taskName + "\" выполнена @" + taskAssigne
	} else {
		return "Задача \"" + taskName + "\" выполнена"
	}

}

// display tasks assigned to this user: /my
func ShowMyTasks(person string) string { 
	var result string
	size := len(TaskManagement)
	if size == 0 {
		return "Нет задач в списке задач"
	}
	for i, j := range TaskManagement {
		if person == j.Assignee {
			if len(result) > 10 {
				result += "\n"
			}
			result = result + strconv.Itoa(i) + ". " + j.Name + " by @" + j.Author
			result = result + "\n/unassign_" + strconv.Itoa(i) + " /resolve_" + strconv.Itoa(i)
		}
	}
	return result
}

// display tasks that were created by this user: /owner
func ShowOwnerTasks(person string) string { 
	var result string
	size := len(TaskManagement)
	if size == 0 {
		return "Нет задач в списке задач"
	}
	for i, j := range TaskManagement {
		if person == j.Author {
			if len(result) > 10 {
				result += "\n"
			}
			result = result + strconv.Itoa(i) + ". " + j.Name + " by @" + j.Author
			result = result + "\n/assign_" + strconv.Itoa(i)
		}
	}
	return result
}

// launching a chatbot
func startTaskBot(ctx context.Context) error { 
	bot, err := tgbotapi.NewBotAPI(BotToken) // initializing BotAPI
	if err != nil {
		log.Fatalf("NewBotAPI failed: %s", err)
	}

	bot.Debug = true // debug
	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)

	_, err = bot.SetWebhook(tgbotapi.NewWebhook(WebhookURL)) // accessing all notifications on WebhookURL
	if err != nil {
		log.Fatalf("SetWebhook failed: %s", err)
	}

	updates := bot.ListenForWebhook("/")

	http.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("all is working"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	go func() {
		log.Fatalln("http err:", http.ListenAndServe(":"+port, nil))
	}()
	fmt.Println("start listen :8081")

	// getting all updates from the updates channel
	for update := range updates {
		UserName := update.Message.From.UserName
		ChatID := update.Message.Chat.ID
		Users[UserName] = ChatID
		Text := update.Message.Text
		command := strings.Split(Text, " ")
		conquer := strings.Split(command[0], "_")
		switch command[0] {
		case "/tasks":
			msg := tgbotapi.NewMessage(ChatID, OutputCurrentTasks(UserName))
			bot.Send(msg)
		case "/new":
			msg := tgbotapi.NewMessage(ChatID, CreatingNewTask(Text[5:], UserName))
			bot.Send(msg)
		case "/my":
			msg := tgbotapi.NewMessage(ChatID, ShowMyTasks(UserName))
			bot.Send(msg)
		case "/owner":
			msg := tgbotapi.NewMessage(ChatID, ShowOwnerTasks(UserName))
			bot.Send(msg)
		default:
			switch conquer[0] {
			case "/assign":
				idd, _ := strconv.Atoi(conquer[1])
				var old int64
				autName := TaskManagement[idd].Author
				if TaskManagement[idd].Assignee == "" {
					old = Users[TaskManagement[idd].Author]
				} else {
					old = Users[TaskManagement[idd].Assignee]
				}
				res1, res2 := SwitchingTaskPerformer(idd, UserName)
				msg := tgbotapi.NewMessage(ChatID, res1)
				bot.Send(msg)
				if autName != UserName {
					msg = tgbotapi.NewMessage(old, res2)
					bot.Send(msg)
				}
			case "/unassign":
				idd, _ := strconv.Atoi(conquer[1])
				res1, res2 := RemovingTaskPerformer(idd, UserName)
				msg := tgbotapi.NewMessage(ChatID, res1)
				bot.Send(msg)
				if res2 != "" {
					msg = tgbotapi.NewMessage(Users[TaskManagement[idd].Author], res2)
					bot.Send(msg)
				}

			case "/resolve":
				idd, _ := strconv.Atoi(conquer[1])
				aut := Users[TaskManagement[idd].Author]
				autName := TaskManagement[idd].Author
				assName := TaskManagement[idd].Assignee
				res1 := ExecutionAndDeletion(idd, UserName)
				msg := tgbotapi.NewMessage(ChatID, res1)
				bot.Send(msg)
				if autName != UserName {
					msg = tgbotapi.NewMessage(aut, res1+" @"+assName)
					bot.Send(msg)
				}
			}
		}

	}
	return nil
}

func main() {
	err := startTaskBot(context.Background())
	if err != nil {
		panic(err)
	}
}
