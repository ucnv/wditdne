package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ucnv/wditdne"
)

const cmdName = "wditdne-hide"

func main() {
	var (
		infile  string
		outfile string
		depth   int
		repeat  bool
		data    io.Reader
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-d <depth> -r -o <outfile>] <infile> <data>\n", cmdName)
		flag.PrintDefaults()
	}
	flag.StringVar(&outfile, "o", "out.jpg", "Filename to outoput.")
	flag.IntVar(&depth, "d", 7, "Number of hidden data in qDCT.")
	flag.BoolVar(&repeat, "r", false, "Repeat the hidden data.")
	flag.Parse()
	if flag.NArg() < 2 {
		exitWithUsage("Specify an input file and data to hide.")
	}
	infile = flag.Arg(0)
	hdata := strings.Join(flag.Args()[1:], " ")

	if _, e := os.Stat(hdata); e != nil {
		s := strings.NewReader(hdata)
		data = s
	} else {
		d, err := os.Open(hdata)
		if err != nil {
			exitWithUsage(err.Error())
		}
		defer d.Close()
		data = d
	}

	in, err := os.Open(infile)
	if err != nil {
		exitWithUsage(err.Error())
	}
	defer in.Close()

	out, err := os.Create(outfile)
	if err != nil {
		exitWithUsage(err.Error())
	}
	defer out.Close()

	// fmt.Println(infile, outfile, depth, repeat, hdata)

	jpeg, err := wditdne.NewJpeg(in)
	if err != nil {
		exitWithUsage(err.Error())
	}
	err = jpeg.Hide(data, depth, repeat, out)
	if err != nil {
		exitWithUsage(err.Error())
	}
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
