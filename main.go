package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Bot parameters
var (
	GuildID  = flag.String("guild", "", "Test guild ID. If not passed - bot registers commands globally")
	BotToken = os.Getenv("DISCORD_TOKEN")
	GPTToken = os.Getenv("GPT_TOKEN")
)

var s *discordgo.Session

func init() { flag.Parse() }

func init() {
	var err error
	if BotToken == "" {
		log.Fatal("Token cannot be empty")
	}

	s, err = discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
}

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "chat",
			Description: "chat with the bot",
			Options: []*discordgo.ApplicationCommandOption{

				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "prompt",
					Description: "The prompt for the bot to respond to",
					Required:    true,
				},
			},
		},
	}
	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"chat": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Get the prompt option from the command parameters
			prompt := i.ApplicationCommandData().Options[0].StringValue()

			// Query the GPT3 API with the prompt
			response, err := queryGPT3(prompt)
			if err != nil {
				log.Printf("Error querying GPT3: %v", err)
				return
			}

			// Trim the newline character from the beginning of the response
			response = response[1:]

			// Send the prompt and response to the channel
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("**Prompt:\n** %v\n**Response:** %v", prompt, response),
				},
			})
		},
	}
)

func init() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func main() {
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Bot is up!")
	})
	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	for _, v := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, *GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
	}

	defer s.Close()

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Gracefully shutdowning; Cleaning up commands")
	for _, v := range commands {
		s.ApplicationCommandDelete(s.State.User.ID, *GuildID, v.Name)
	}
}

func queryGPT3(prompt string) (response string, err error) {
	// Define the request body
	type GPT3Request struct {
		Model       string  `json:"model"`
		Prompt      string  `json:"prompt"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
	}

	// Set up the HTTP request to the GPT3 API
	url := "https://api.openai.com/v1/completions"
	reqBody := &GPT3Request{
		Model:       "gpt-3.5-turbo",
		Prompt:      prompt,
		MaxTokens:   128,
		Temperature: 0.9,
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+GPTToken)

	// Send the request to the GPT3 API
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to GPT3 API: %v", err)
		return "", err
	}

	// Read the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	// Parse the response
	var gpt3Response map[string]interface{}
	if err := json.Unmarshal(body, &gpt3Response); err != nil {
		return "", err
	}

	// Get the "choices" field from the response
	choices, ok := gpt3Response["choices"].([]interface{})
	if !ok {
		return "", fmt.Errorf("error: choices field is not an array")
	}

	// Get the first object in the "choices" array
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("error: first element of choices array is not an object")
	}

	// Get the "text" field from the object
	text, ok := choice["text"].(string)
	if !ok {
		return "", fmt.Errorf("error: text field in first element of choices array is not a string")
	}

	// Return the "text" field
	return text, nil
}
