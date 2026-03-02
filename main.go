package main

import (
	"flag"
	"fmt"
	"os"
)

var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("openclaw-config %s\n", Version)
		os.Exit(0)
	}

	app := NewApp()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
