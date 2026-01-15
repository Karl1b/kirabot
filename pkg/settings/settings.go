package settings

import (
	"fmt"

	"github.com/DeanPDX/dotconfig"
)

var Settings Go4lageSettings

func init() {
	var err error
	Settings, err = dotconfig.FromFileName[Go4lageSettings](".env")
	if err != nil {
		fmt.Printf("Error: %v.", err)
	}
}

type Go4lageSettings struct {
	TelegramToken string `env:"TELEGRAMTOKEN"`
	LlmKey        string `env:"LLMKEY"`
}
