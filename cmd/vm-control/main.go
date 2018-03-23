package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Symantec/Dominator/lib/constants"
	"github.com/Symantec/Dominator/lib/flagutil"
	"github.com/Symantec/Dominator/lib/log"
	"github.com/Symantec/Dominator/lib/log/cmdlogger"
	"github.com/Symantec/Dominator/lib/srpc/setupclient"
	"github.com/Symantec/Dominator/lib/tags"
)

var (
	clusterManagerHostname = flag.String("clusterManagerHostname", "localhost",
		"Hostname of Cluster Resource Manager")
	clusterManagerPortNum = flag.Uint("clusterManagerPortNum",
		constants.ClusterManagerPortNumber,
		"Port number of Cluster Resource Manager")
	hypervisorHostname = flag.String("hypervisorHostname", "",
		"Hostname of hypervisor")
	hypervisorPortNum = flag.Uint("hypervisorPortNum",
		constants.HypervisorPortNumber, "Port number of hypervisor")
	imageFile = flag.String("imageFile", "",
		"Name of RAW image file to boot with")
	imageName    = flag.String("imageName", "", "Name of image to boot with")
	imageTimeout = flag.Duration("imageTimeout", time.Minute,
		"Time to wait before timing out on image fetch")
	imageURL = flag.String("imageURL", "",
		"Name of URL of image to boot with")
	memory       = flag.Uint64("memory", 128, "memory in MiB")
	milliCPUs    = flag.Uint("milliCPUs", 250, "milli CPUs")
	minFreeBytes = flag.Uint64("minFreeBytes", 64<<20,
		"minimum number of free bytes in root volume")
	ownerGroups          flagutil.StringList
	ownerUsers           flagutil.StringList
	secondaryVolumeSizes flagutil.StringList
	subnetId             = flag.String("subnetId", "",
		"Subnet ID to launch VM in")
	responseTimeout = flag.Duration("responseTimeout", time.Minute,
		"Time to wait before timing out on network response from VM")
	roundupPower = flag.Uint64("roundupPower", 24,
		"power of 2 to round up root volume size")
	userDataFile = flag.String("userDataFile", "",
		"Name file containing user-data accessible from the metadata server")
	vmTags tags.Tags

	logger log.DebugLogger
)

func init() {
	flag.Var(&ownerGroups, "ownerGroups", "Groups who own the VM")
	flag.Var(&ownerUsers, "ownerUsers", "Extra users who own the VM")
	flag.Var(&secondaryVolumeSizes, "secondaryVolumeSizes",
		"Sizes for secondary volumes")
	flag.Var(&vmTags, "vmTags", "Tags to apply to VM")
}

func printUsage() {
	fmt.Fprintln(os.Stderr,
		"Usage: vm-control [flags...] command [args...]")
	fmt.Fprintln(os.Stderr, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  add-address MACaddr IPaddr")
	fmt.Fprintln(os.Stderr, "  add-subnet ID IPgateway IPmask DNSserver...")
	fmt.Fprintln(os.Stderr, "  change-vm-tags IPaddr")
	fmt.Fprintln(os.Stderr, "  create-vm")
	fmt.Fprintln(os.Stderr, "  destroy-vm IPaddr")
	fmt.Fprintln(os.Stderr, "  get-vm-info IPaddr")
	fmt.Fprintln(os.Stderr, "  replace-vm-image IPaddr")
	fmt.Fprintln(os.Stderr, "  restore-vm-image IPaddr")
	fmt.Fprintln(os.Stderr, "  start-vm IPaddr")
	fmt.Fprintln(os.Stderr, "  stop-vm IPaddr")
}

type commandFunc func([]string, log.DebugLogger)

type subcommand struct {
	command string
	minArgs int
	maxArgs int
	cmdFunc commandFunc
}

var subcommands = []subcommand{
	{"add-address", 2, 2, addAddressSubcommand},
	{"add-subnet", 4, -1, addSubnetSubcommand},
	{"change-vm-tags", 1, 1, changeVmTagsSubcommand},
	{"create-vm", 0, 0, createVmSubcommand},
	{"destroy-vm", 1, 1, destroyVmSubcommand},
	{"get-vm-info", 1, 1, getVmInfoSubcommand},
	{"replace-vm-image", 1, 1, replaceVmImageSubcommand},
	{"restore-vm-image", 1, 1, restoreVmImageSubcommand},
	{"start-vm", 1, 1, startVmSubcommand},
	{"stop-vm", 1, 1, stopVmSubcommand},
}

func main() {
	flag.Usage = printUsage
	flag.Parse()
	if flag.NArg() < 1 {
		printUsage()
		os.Exit(2)
	}
	logger = cmdlogger.New()
	if *clusterManagerHostname == "" && *hypervisorHostname == "" {
		fmt.Fprintln(os.Stderr, "no-one to talk to")
		os.Exit(2)
	}
	if err := setupclient.SetupTls(true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	numSubcommandArgs := flag.NArg() - 1
	for _, subcommand := range subcommands {
		if flag.Arg(0) == subcommand.command {
			if numSubcommandArgs < subcommand.minArgs ||
				(subcommand.maxArgs >= 0 &&
					numSubcommandArgs > subcommand.maxArgs) {
				printUsage()
				os.Exit(2)
			}
			subcommand.cmdFunc(flag.Args()[1:], logger)
			os.Exit(3)
		}
	}
	printUsage()
	os.Exit(2)
}
