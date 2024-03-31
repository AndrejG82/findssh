package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kevinburke/ssh_config"
	"github.com/manifoldco/promptui"
)

const defaultUser = "root"

type Element struct {
	Name     string
	Hostname string
	User     string
}

func createElements(sshConfigPath string) []Element {
	f, _ := os.Open(sshConfigPath)
	cfg, err := ssh_config.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not parse config: %v", err)
		os.Exit(1)
	}

	// create list of elements
	var elements []Element
	for _, host := range cfg.Hosts {
		if len(host.Patterns) == 1 && host.Patterns[0].String() == "*" {
			continue
		}

		var el Element
		for _, pat := range host.Patterns {
			el.Name = pat.String()
			break
		}

		for _, node := range host.Nodes {
			switch node.(type) {
			case *ssh_config.KV:
				kv := node.(*ssh_config.KV)
				if kv.Key == "HostName" {
					el.Name = el.Name + " " + kv.Value
					el.Hostname = kv.Value
				}
				if kv.Key == "User" {
					el.User = kv.Value
				}
			}
		}
		if el.User == "" {
			el.User = defaultUser
		}
		elements = append(elements, el)

	}

	return elements
}

func openTerminalAndSSH(hostname string, user string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`tell application "Terminal"
            activate           
            do script "ssh %s@%s"
            end tell`, user, hostname)
		cmd = exec.Command("osascript", "-e", script)
	case "windows":
		cmd = exec.Command("cmd", "/C", fmt.Sprintf("start ssh %s@%s", user, hostname))
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

func searchElementsByName(elements []Element, name string) []Element {
	var matched []Element
	for _, e := range elements {
		if strings.Contains(e.Name, name) {
			matched = append(matched, e)
		}
	}
	return matched
}

func promptUserToSelectElement(elements []Element) (Element, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   "\U0001F4BB {{ .Name | cyan }} ({{ .User | red }}@{{.Hostname | red }})",
		Inactive: "  {{ .Name | white }} ({{ .User | white }}@{{.Hostname | white }})",
	}

	searcher := func(input string, index int) bool {
		element := elements[index]
		name := strings.Replace(strings.ToLower(element.Name), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:     "Select host",
		Items:     elements,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
	}
	i, _, err := prompt.Run()

	if err != nil {
		return Element{}, err
	}

	return elements[i], nil
}

func main() {
	var searchTerm string = ""
	if len(os.Args) > 1 {
		searchTerm = strings.Join(os.Args[1:], " ")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding the home directory:", err)
		return
	}
	var sshConfigPath string = filepath.Join(homeDir, ".ssh", "config")

	var elements = createElements(sshConfigPath)

	/*
		for _, el := range elements {
			fmt.Printf("Name: %s, HostName: %s, User: %s\n", el.name, el.hostname, el.user)
		}
	*/

	// search elements
	matchedElements := searchElementsByName(elements, searchTerm)

	switch len(matchedElements) {
	case 0:
		fmt.Println("Host not found")
	case 1:
		openTerminalAndSSH(matchedElements[0].Hostname, matchedElements[0].User)
	default:
		element, err := promptUserToSelectElement(matchedElements)
		if err != nil {
			fmt.Printf("EXIT:: %v\n", err)
			return
		}
		openTerminalAndSSH(element.Hostname, element.User)
	}

}
