package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/service"
)

// confirm asks before an irreversible change. It returns false when there is no
// terminal to ask on, so a script that reaches this by accident stops rather
// than proceeding unattended.
func confirm(prompt string) bool {
	fmt.Print(prompt)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fmt.Println()
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// resetAdmin restores the first admin account.
//
// Interactively it asks for a new password and never echoes it back. With
// -yes (assumeYes) it skips the prompt and resets to admin/admin, the
// historical scripted behaviour: a mistyped unattended command would
// otherwise leave a live panel with the best-known default credentials in
// the world, reachable from the internet.
func resetAdmin(assumeYes bool) {
	newPassword := "admin"
	if !assumeYes {
		fmt.Println("This resets the first admin account to the username \"admin\".")
		fmt.Println("Anyone who can reach the panel will be able to log in with the new password.")
		if !confirm("Type y to continue: ") {
			fmt.Println("cancelled")
			return
		}
		fmt.Print("Enter new password: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			newPassword = strings.TrimSpace(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("error reading password:", err)
			return
		}
		if newPassword == "" {
			fmt.Println("Password cannot be empty")
			return
		}
	}

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	userService := service.UserService{}
	err = userService.UpdateFirstUser("admin", newPassword)
	if err != nil {
		fmt.Println("reset admin credentials failed:", err)
		return
	}
	if assumeYes {
		fmt.Println("reset admin credentials success")
		fmt.Println("Change them now: s-ui admin -username <user> -password <pass>")
	} else {
		fmt.Println("============================================")
		fmt.Println(" Admin credentials have been reset!")
		fmt.Println(" Username: admin")
		fmt.Println(" Password: (updated, not echoed)")
		fmt.Println("============================================")
	}
}

func updateAdmin(username string, password string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	if username != "" || password != "" {
		userService := service.UserService{}
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println("reset admin credentials failed:", err)
		} else {
			fmt.Println("reset admin credentials success")
		}
	}
}

func showAdmin() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	userService := service.UserService{}
	userModel, err := userService.GetFirstUser()
	if err != nil {
		fmt.Println("get current user info failed,error info:", err)
	}
	username := userModel.Username
	userpasswd := userModel.Password
	if (username == "") || (userpasswd == "") {
		fmt.Println("current username or password is empty")
	}
	fmt.Println("First admin credentials:")
	fmt.Println("\tUsername:\t", username)
	fmt.Println("\tPassword:\t <hashed, not recoverable>")
	fmt.Println("To set a new password, run: s-ui admin -username <user> -password <pass>")
}
