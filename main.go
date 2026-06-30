package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

var Version = "1.2.3"

func main() {
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("openclaw-config %s\n", displayVersion(Version))
		os.Exit(0)
	}

	app, err := NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		if errors.Is(err, ErrUserCancelled) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
