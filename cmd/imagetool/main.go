package main

import (
	"flag"
	"fmt"
	"github.com/Symantec/Dominator/lib/constants"
	"github.com/Symantec/Dominator/lib/filesystem"
	"github.com/Symantec/Dominator/lib/filter"
	"github.com/Symantec/Dominator/lib/objectclient"
	"github.com/Symantec/Dominator/lib/srpc"
	"net/rpc"
	"os"
	"path"
)

var (
	buildLog = flag.String("buildLog", "",
		"Filename or URL containing build log")
	certFile = flag.String("certFile",
		path.Join(os.Getenv("HOME"), ".ssl/cert.pem"),
		"Name of file containing the user SSL certificate")
	debug = flag.Bool("debug", false,
		"If true, show debugging output")
	filterFile = flag.String("filterFile", "",
		"Filter file to apply when diffing images")
	imageServerHostname = flag.String("imageServerHostname", "localhost",
		"Hostname of image server")
	imageServerPortNum = flag.Uint("imageServerPortNum",
		constants.ImageServerPortNumber,
		"Port number of image server")
	keyFile = flag.String("keyFile",
		path.Join(os.Getenv("HOME"), ".ssl/key.pem"),
		"Name of file containing the user SSL key")
	releaseNotes = flag.String("releaseNotes", "",
		"Filename or URL containing release notes")
	skipFields = flag.String("skipFields", "",
		"Fields to skip when showing or diffing images")
)

func printUsage() {
	fmt.Fprintln(os.Stderr,
		"Usage: imagetool [flags...] add|check|delete|list [args...]")
	fmt.Fprintln(os.Stderr, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  add    name imagefile filterfile triggerfile")
	fmt.Fprintln(os.Stderr, "  check  name")
	fmt.Fprintln(os.Stderr, "  delete name")
	fmt.Fprintln(os.Stderr, "  diff   tool left right")
	fmt.Fprintln(os.Stderr, "         left & right are image sources. Format:")
	fmt.Fprintln(os.Stderr, "         type:name where type is one of:")
	fmt.Fprintln(os.Stderr, "           f: name of file containing an image")
	fmt.Fprintln(os.Stderr, "           i: name of an image on the imageserver")
	fmt.Fprintln(os.Stderr, "           s: name of sub to poll")
	fmt.Fprintln(os.Stderr, "  get    name directory")
	fmt.Fprintln(os.Stderr, "  list")
	fmt.Fprintln(os.Stderr, "  show   name")
	fmt.Fprintln(os.Stderr, "Fields:")
	fmt.Fprintln(os.Stderr, "  m: mode")
	fmt.Fprintln(os.Stderr, "  l: number of hardlinks")
	fmt.Fprintln(os.Stderr, "  u: UID")
	fmt.Fprintln(os.Stderr, "  g: GID")
	fmt.Fprintln(os.Stderr, "  s: size/Rdev")
	fmt.Fprintln(os.Stderr, "  t: time of last modification")
	fmt.Fprintln(os.Stderr, "  n: name")
	fmt.Fprintln(os.Stderr, "  d: data (hash or symlink target)")
}

type commandFunc func([]string)

type subcommand struct {
	command string
	numArgs int
	cmdFunc commandFunc
}

var subcommands = []subcommand{
	{"add", 4, addImageSubcommand},
	{"check", 1, checkImageSubcommand},
	{"delete", 1, deleteImageSubcommand},
	{"diff", 3, diffSubcommand},
	{"get", 2, getImageSubcommand},
	{"list", 0, listImagesSubcommand},
	{"show", 1, showImageSubcommand},
}

var imageRpcClient *rpc.Client
var imageSrpcClient *srpc.Client
var theObjectClient *objectclient.ObjectClient

var listSelector filesystem.ListSelector

func getClients() (*rpc.Client, *srpc.Client, *objectclient.ObjectClient) {
	if imageRpcClient == nil {
		var err error
		clientName := fmt.Sprintf("%s:%d",
			*imageServerHostname, *imageServerPortNum)
		imageRpcClient, err = rpc.DialHTTP("tcp", clientName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error dialing\t%s\n", err)
			os.Exit(1)
		}
		imageSrpcClient, err = srpc.DialHTTP("tcp", clientName, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error dialing\t%s\n", err)
			os.Exit(1)
		}
		theObjectClient = objectclient.NewObjectClient(clientName)
	}
	return imageRpcClient, imageSrpcClient, theObjectClient
}

func makeListSelector(arg string) filesystem.ListSelector {
	var mask filesystem.ListSelector = filesystem.ListSelectAll
	for _, char := range arg {
		switch char {
		case 'm':
			mask |= filesystem.ListSelectSkipMode
		case 'l':
			mask |= filesystem.ListSelectSkipNumLinks
		case 'u':
			mask |= filesystem.ListSelectSkipUid
		case 'g':
			mask |= filesystem.ListSelectSkipGid
		case 's':
			mask |= filesystem.ListSelectSkipSizeDevnum
		case 't':
			mask |= filesystem.ListSelectSkipMtime
		case 'n':
			mask |= filesystem.ListSelectSkipName
		case 'd':
			mask |= filesystem.ListSelectSkipData
		}
	}
	return mask
}

var listFilter *filter.Filter

func main() {
	flag.Usage = printUsage
	flag.Parse()
	if flag.NArg() < 1 {
		printUsage()
		os.Exit(2)
	}
	listSelector = makeListSelector(*skipFields)
	var err error
	if *filterFile != "" {
		listFilter, err = filter.LoadFilter(*filterFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	setupTls(*certFile, *keyFile)
	for _, subcommand := range subcommands {
		if flag.Arg(0) == subcommand.command {
			if flag.NArg()-1 != subcommand.numArgs {
				printUsage()
				os.Exit(2)
			}
			subcommand.cmdFunc(flag.Args()[1:])
			os.Exit(3)
		}
	}
	printUsage()
	os.Exit(2)
}
