package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

var Version = "1.3.0"

func main() {
	restore := initWindowsConsole()
	defer restore()
	detectCJKLocale()

	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("openclaw-config v%s\n", Version)
		os.Exit(0)
	}

	app, err := NewApp()
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		if errors.Is(err, ErrUserCancelled) {
			return
		}
		printError(err.Error())
		os.Exit(1)
	}
}
