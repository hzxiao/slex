package main

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"os"
	"os/signal"
)

const version = "0.0.1"

var (
	srv     = flag.BoolP("server", "s", false, "running as server mode")
	cli     = flag.BoolP("client", "c", false, "running as client mode")
	ver     = flag.BoolP("version", "v", false, "print slex version")
	help    = flag.BoolP("help", "h", false, "show usage of slex command")
	cfgName = flag.StringP("config", "f", "./slex.yaml", "yaml config file")
)

func main() {
	flag.Parse()

	if *ver {
		printVersion()
		exit(0)
	}

	if *help {
		printUsage()
		exit(0)
	}

	if *srv && *cli {
		fmt.Println("invalid flag: you cannot run mode Server and mode Client at the same time")
		exit(1)
	}
	if !*srv && !*cli {
		fmt.Println("lack of flag, you should run as server mode or client mode")
		fmt.Println()
		printUsage()
		exit(1)
	}

	wc := make(chan os.Signal, 1)
	signal.Notify(wc, os.Interrupt, os.Kill)
	<-wc
	fmt.Println("close slex...")
}

func printVersion() {
	fmt.Printf("slex version slex%v\n", version)
}

func printUsage() {
	flag.Usage()
}

func exit(code int) {
	os.Exit(code)
}
