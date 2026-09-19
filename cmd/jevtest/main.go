package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"

	"jev-decision-client/jev"
)

func printAnswer(name string, raw json.RawMessage) {
	ans := jev.ParseAnswer(raw)
	fmt.Printf("\nDecision for '%s' (type: %s)\n", name, ans.Type)

	if ans.Noul != nil {
		fmt.Printf("  true:  %.2f%%\n", *ans.Noul*100)
		fmt.Printf("  false: %.2f%%\n", (1-*ans.Noul)*100)
	}
	if ans.Choice != "" {
		fmt.Printf("  choice: %s\n", ans.Choice)
	}
	if ans.Score != nil {
		fmt.Printf("  score: %.2f\n", *ans.Score)
	}
	for opt, p := range ans.Probabilities {
		fmt.Printf("    - %s: %.2f%%\n", opt, p*100)
	}
}

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Println("Error: OPENROUTER_API_KEY is not set in .env or system environment.")
		os.Exit(1)
	}

	payload := jev.DecisionRequest{
		Model: "~typesafe/jev-latest",
		State: "My credit card was charged twice for the subscription.",
		Questions: map[string]jev.Question{
			"is_urgent": {
				Type:         "noul",
				Instructions: "Is the customer experiencing a critical billing issue?",
				Criteria: map[string]string{
					"true":  "Explicitly time-sensitive or double billing issues",
					"false": "No immediate financial or technical risk",
				},
			},
		},
	}

	client := jev.NewClient(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Decide(ctx, payload)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	pretty, err := json.MarshalIndent(resp, "", "  ")
	if err == nil {
		if err := os.WriteFile("jev_decision.json", pretty, 0644); err != nil {
			fmt.Printf("Warning: Could not save jev_decision.json: %v\n", err)
		} else {
			fmt.Println("Successfully saved parsed result to jev_decision.json")
		}
	}

	if len(resp.Answers) == 0 {
		fmt.Println("\nNo answers returned by the API.")
		return
	}
	for name, raw := range resp.Answers {
		printAnswer(name, raw)
	}
}
