package main

import (
	"context"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

func loadSystemPrompt(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func appendPrompt(filename, username, prompt, response string) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("❌ Gagal menulis ke system_prompt.txt:", err)
		return
	}
	defer f.Close()

	entry := "[User @" + username + "]: " + prompt + "\n[AI]: " + response + "\n\n"
	_, err = f.WriteString(entry)
	if err != nil {
		log.Println("Gagal menambahkan prompt:", err)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Gagal load .env:", err)
	}

	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	if tgToken == "" || apiKey == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN atau OPENROUTER_API_KEY belum diisi.")
	}

	bot, err := tgbotapi.NewBotAPI(tgToken)
	if err != nil {
		log.Fatal("Gagal inisialisasi Telegram bot:", err)
	}

	log.Printf("🤖 Bot aktif sebagai: %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, _ := bot.GetUpdatesChan(u)

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"
	client := openai.NewClientWithConfig(config)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userPrompt := update.Message.Text
		chatID := update.Message.Chat.ID
		username := update.Message.From.UserName

		log.Printf("📨 [ID:%d | @%s]: %s", update.Message.From.ID, username, userPrompt)

		reply, err := askOpenRouter(client, userPrompt)
		if err != nil {
			reply = "Maaf, AI tidak bisa menjawab saat ini."
			log.Println("Error OpenRouter:", err)
		}

		appendPrompt("system_prompt.txt", username, userPrompt, reply)

		msg := tgbotapi.NewMessage(chatID, reply)
		bot.Send(msg)
	}
}

func askOpenRouter(client *openai.Client, prompt string) (string, error) {
	systemPrompt, err := loadSystemPrompt("system_prompt.txt")
	if err != nil {
		log.Println("⚠️ Gagal membaca system prompt dari file:", err)
		systemPrompt = "Kamu adalah asisten chatbot."
	}

	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: "mistralai/mistral-7b-instruct",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
