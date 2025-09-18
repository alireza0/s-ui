package cmd

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/alireza0/s-ui/cmd/migration"
	"github.com/alireza0/s-ui/config"
)

func ParseCmd() {
	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")

	adminCmd := flag.NewFlagSet("admin", flag.ExitOnError)
	settingCmd := flag.NewFlagSet("setting", flag.ExitOnError)

	var username string
	var password string
	var port int
	var path string
	var subPort int
	var subPath string
	var reset bool
	var show bool
	settingCmd.BoolVar(&reset, "reset", false, "reset all settings")
	settingCmd.BoolVar(&show, "show", false, "show current settings")
	settingCmd.IntVar(&port, "port", 0, "set panel port")
	settingCmd.StringVar(&path, "path", "", "set panel path")
	settingCmd.IntVar(&subPort, "subPort", 0, "set sub port")
	settingCmd.StringVar(&subPath, "subPath", "", "set sub path")

	adminCmd.BoolVar(&show, "show", false, "show first admin credentials")
	adminCmd.BoolVar(&reset, "reset", false, "reset first admin credentials")
	adminCmd.StringVar(&username, "username", "", "set login username")
	adminCmd.StringVar(&password, "password", "", "set login password")

	oldUsage := flag.Usage
	flag.Usage = func() {
		oldUsage()
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("    admin          set/reset/show first admin credentials")
		fmt.Println("    uri            Show panel URI")
		fmt.Println("    migrate        migrate form older version")
		fmt.Println("    setting        set/reset/show settings")
		fmt.Println()
		adminCmd.Usage()
		fmt.Println()
		settingCmd.Usage()
	}

	flag.Parse()
	if showVersion {
		fmt.Println("S-UI Panel\t", config.GetVersion())
		info, ok := debug.ReadBuildInfo()
		if ok {
			for _, dep := range info.Deps {
				if dep.Path == "github.com/sagernet/sing-box" {
					fmt.Println("Sing-Box\t", dep.Version)
					break
				}
			}
		}
		return
	}

	switch os.Args[1] {
	case "admin":
		err := adminCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		switch {
		case show:
			showAdmin()
		case reset:
			resetAdmin()
		default:
			updateAdmin(username, password)
			showAdmin()
		}

	case "uri":
		getPanelURI()

	case "migrate":
		migration.MigrateDb()

	case "setting":
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		switch {
		case show:
			showSetting()
		case reset:
			resetSetting()
		default:
			updateSetting(port, path, subPort, subPath)
			showSetting()
		}
	default:
		fmt.Println("Invalid subcommands")
		flag.Usage()
	}
}
