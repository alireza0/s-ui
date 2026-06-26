package cmd

import (
	"fmt"
	"os"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database"
)

func backupDb(output string, exclude string) {
	if output == "" {
		fmt.Println("backup failed: -output is required (use - for stdout)")
		return
	}
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Println("backup failed:", err)
		return
	}
	data, err := database.GetDb(exclude)
	if err != nil {
		fmt.Println("backup failed:", err)
		return
	}
	if output == "-" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintln(os.Stderr, "backup failed:", err)
		}
		return
	}
	if err := os.WriteFile(output, data, 0600); err != nil {
		fmt.Println("backup failed:", err)
		return
	}
	fmt.Println("backup saved to", output)
}
