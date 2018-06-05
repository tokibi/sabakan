package main

import (
	"context"
	"flag"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/cybozu-go/cmd"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/sabakan/client"
	"github.com/google/subcommands"
)

var (
	flagServer = flag.String("server", "http://localhost:10080", "<Listen IP>:<Port number>")
)

var discardLogger *log.Logger

func init() {
	discardLogger = log.NewLogger()
	discardLogger.SetOutput(ioutil.Discard)
}

func main() {
	flag.Parse()

	c := client.NewClient(*flagServer, &cmd.HTTPClient{
		Severity: log.LvDebug,
		Client:   &http.Client{},
		Logger:   discardLogger,
	})

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(dhcpCommand(c), "")
	subcommands.Register(ipamCommand(c), "")
	subcommands.Register(machinesCommand(c), "")
	subcommands.Register(imagesCommand(c), "")
	subcommands.Register(ignitionsCommand(c), "")

	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
