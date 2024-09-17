package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ucnv/wditdne"
)

const cmdName = "wditdne-extract"

func main() {
	var (
		infile  string
		data    string
		verbose bool
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <infile>\n", cmdName)
		flag.PrintDefaults()
	}
	flag.BoolVar(&verbose, "v", false, "Show all quantized DCT coefficients.")
	flag.Parse()
	if flag.NArg() < 1 {
		exitWithUsage("Specify an input file.")
	}
	infile = flag.Arg(0)

	in, err := os.Open(infile)
	if err != nil {
		exitWithUsage(err.Error())
	}
	defer in.Close()

	jpeg, err := wditdne.NewJpeg(in)
	if err != nil {
		exitWithUsage(err.Error())
	}

	data, err = jpeg.Extract(verbose)
	if err != nil {
		exitWithUsage(err.Error())
	}
	fmt.Print(string(data))
}

func exitWithUsage(messages ...string) {
	if len(messages) > 0 {
		for _, v := range messages {
			fmt.Fprintln(os.Stderr, v)
		}
		fmt.Fprintln(os.Stderr, "")
	}
	flag.Usage()
	os.Exit(1)
}
