package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/solguardlabs/compassdtl/src/api"
	"github.com/solguardlabs/compassdtl/src/report"
	"github.com/solguardlabs/compassdtl/src/scenario"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		runScenario(os.Args[2:])
	case "serve":
		serve(os.Args[2:])
	case "snapshot-default":
		writeDefault()
	default:
		usage()
		os.Exit(2)
	}
}

func runScenario(args []string) {
	flags := flag.NewFlagSet("run", flag.ExitOnError)
	if err := flags.Parse(args); err != nil {
		log.Fatal(err)
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "run requires a fixture path")
		os.Exit(2)
	}
	definition, err := scenario.LoadFile(flags.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	result, err := scenario.Run(definition)
	if err != nil {
		log.Fatal(err)
	}
	if err := report.WriteScenarioResult(os.Stdout, result); err != nil {
		log.Fatal(err)
	}
}

func serve(args []string) {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := flags.String("addr", "127.0.0.1:8087", "listen address")
	configPath := flags.String("config", "", "bootstrap or scenario fixture")
	if err := flags.Parse(args); err != nil {
		log.Fatal(err)
	}
	bootstrap := scenario.DefaultBootstrap()
	if *configPath != "" {
		loaded, err := scenario.LoadBootstrapFile(*configPath)
		if err != nil {
			log.Fatal(err)
		}
		bootstrap = loaded
	}
	service, err := api.NewService(bootstrap)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("CompassDTL listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, api.NewHTTPHandler(service)))
}

func writeDefault() {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(scenario.DefaultBootstrap()); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  compassdtl run <scenario.json>")
	fmt.Fprintln(os.Stderr, "  compassdtl serve [--addr 127.0.0.1:8087] [--config fixture.json]")
	fmt.Fprintln(os.Stderr, "  compassdtl snapshot-default")
}
