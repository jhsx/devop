// Copyright 2016 José Santos <henrique_1609@me.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

var (
	_port           = flag.String("p", "", "-p=\"8080:8888\" specifies dev and serve ports")
	_debug          = flag.Bool("v", false, "-v enter verbose mode")
	_tickerDuration = flag.String("t", "", "-t=1s set's a timeout to check for changes and re-run the commands if needed")
)

func trace(f string, v ...interface{}) {
	log.Printf(f, v...)
}

func debug(format string, v ...interface{}) {
	if *_debug {
		log.Printf(format, v...)
	}
}

var devService Service
var commands map[string]*command
var appHost string
var root string

func main() {

	flag.Parse()
	trace("devop development server started")
	devopfile, err := ioutil.ReadFile("devop.yml")
	if err != nil {
		trace("can't load devop.yml, error: %s", err)
		return
	}

	debug("parsing devop yml file")
	yaml.Unmarshal(devopfile, &devService)

	trace("initializing commands and configs")
	devService.Init()

	appHost = fmt.Sprintf("%s:%s", "127.0.0.1", devService.AppPort)

	root = devService.GetRoot()
	commands = devService.Commands

	trace("commands are loaded")
	trace("running initial command scan")
	runCommands(scanAndGetCommands(root, commands), commands)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
		<-c
		for commandName, command := range commands {
			command.forceKillAllProcess()
			if command.Onexit != "" {
				trace("running on exit command of %s", commandName)
				cmd := newProcessCommand(command.Onexit)
				cmd.Env = command.Env
				cmd.Run()
			}
		}
		trace("devop is exiting")
		os.Exit(1)
	}()

	var dialer = &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	trace("starting file system modifications tracker")
	go trackModifications()

	trace("starting auto-refresh daemon")
	autoRefresher()

	if devService.DevPort != "" {
		trace("starting proxy server on: http://localhost:%v -> http://localhost:%v", devService.DevPort, devService.AppPort)
		http.ListenAndServe(fmt.Sprintf(":%s", devService.DevPort), &httputil.ReverseProxy{
			Director: director,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				Dial: func(network, address string) (net.Conn, error) {
					con, err := dialer.Dial(network, address)
					if err != nil {
						var i = 0
						for {
							<-time.After(time.Millisecond * 10)
							con, err = dialer.Dial(network, address)
							if err == nil || i > 500 {
								break
							}
							i++
						}
					}
					return con, err
				},
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		})
	} else {
		<-(chan struct{})(nil)
	}
}

func autoRefresher() {
	duration, err := time.ParseDuration(devService.Refresh)
	if err != nil {
		panic(err)
	}
	go func() {
		for range time.Tick(duration) {
			runCommandsIfneeded()
		}
	}()
}

var pendingCommands = make(map[string]*command)

var pendingMx = sync.Mutex{}
var runMutex = sync.Mutex{}

func runCommandsIfneeded() {
	pendingMx.Lock()
	commandsToRun := pendingCommands
	hasCommands := len(commandsToRun) > 0
	if hasCommands {
		pendingCommands = make(map[string]*command)
	}
	pendingMx.Unlock()
	runMutex.Lock()
	defer runMutex.Unlock()
	if hasCommands {
		runCommands(commandsToRun, commands)
	}
}
func director(req *http.Request) {
	req.URL.Host = appHost
	req.URL.Scheme = "http"
	runCommandsIfneeded()
}

func killCommand(cmdString string, command *command, commandRoot map[string]*command) {
	if _, ok := command.running[cmdString]; ok {

		if command.Continue != "" {
			if command, has := commandRoot[command.Continue]; has && command.Wait == false {
				killCommand(command.Command, command, commandRoot)
			}
		}

		trace("killing command: %s", cmdString)
		command.forceKillProcess(cmdString)
	}
}

// runCommand runs a single command and all it's continuations
// if command has a command continuation it's will be invoked
func runCommand(cmdString string, command *command, commandRoot map[string]*command) {

	if command.Wait == false {
		killCommand(cmdString, command, commandRoot)
	}

	cmd := newProcessCommand(cmdString)
	cmd.Env = command.Env
	if command.Dir != "" {
		cmd.Dir = command.Dir
	}

	if command.Stderr {
		cmd.Stderr = os.Stderr
	}

	if command.Stdout {
		cmd.Stdout = os.Stdout
	}

	if !command.Wait {
		//cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		// if command kill option is activated the command will be stored and killed in next match run
		command.running[cmdString] = cmd
	}

	trace("running command: %s", cmdString)
	err := cmd.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}

	if command.Wait {
		err = cmd.Wait()
		if err != nil {
			log.Println(err.Error())
			return
		}
	}

}

func BreakCommandString(commandStr string) []string {
	var (
		commandBreak []string
		lexState     = 0
		lexStart     = 0
		scapeFound   = false
	)
	const lexNone = 0
	const lexName = 1
	const lexStringSingle = 2
	const lexString = 3

	for pos, _rune := range commandStr {

		if lexState == lexNone {
			switch _rune {
			case '\t', ' ':
			case '"':
				lexState = lexString
				lexStart = pos
			case '\'':
				lexState = lexStringSingle
				lexStart = pos
			default:
				lexState = lexName
				lexStart = pos
			}
		} else if lexState == lexString {

			if scapeFound {
				scapeFound = false
				continue
			}

			switch _rune {
			case '"':
				_break, err := unQuote(commandStr[lexStart : pos+1])
				if err != nil {
					trace("unexpected error parsing command string: %s", err)
					os.Exit(0)
				}
				commandBreak = append(commandBreak, _break)
				lexState = lexNone
				lexStart = pos
			case '\\':
				scapeFound = true
			}

		} else if lexState == lexStringSingle {

			if scapeFound {
				scapeFound = false
				continue
			}

			switch _rune {
			case '\'':
				_break, err := unQuote(commandStr[lexStart : pos+1])
				if err != nil {
					trace("unexpected error parsing command string: %s", err)
					os.Exit(0)
				}
				commandBreak = append(commandBreak, _break)
				lexState = lexNone
				lexStart = pos
			case '\\':
				scapeFound = true
			}

		} else if lexState == lexName {
			switch _rune {
			case '\t', ' ':
				commandBreak = append(commandBreak, commandStr[lexStart:pos])
				lexStart = pos
				lexState = lexNone
			}
		}
	}

	if lexState == lexName {
		commandBreak = append(commandBreak, commandStr[lexStart:])
	} else if lexState != lexNone {
		trace("unexpected error command string: unclosed string literal")
		os.Exit(0)
	}

	return commandBreak
}

// runCommands runs a list of commands, commandMap is a map of commandString and *command,
// this function should be invoked with the first argument result of scanAndGetCommands
// commandRoot is the map of all commands available is used when a command has continuation
func runCommands(commandMap map[string]*command, commandRoot map[string]*command) {
	for cmdString, cmd := range commandMap {
		runCommand(cmdString, cmd, commandRoot)
	}

	var _commandMap map[string]*command
	for _, _command := range commandMap {
		if _command.Continue != "" {
			if _commandMap == nil {
				_commandMap = map[string]*command{}
			}
			if cmd, has := commandRoot[_command.Continue]; has {
				_commandMap[cmd.Command] = cmd
			}
		}
	}
	if _commandMap != nil {
		runCommands(_commandMap, commandRoot)
	}
}

func matchCommands(commandsRun map[string]*command, path string) {
	for commandName, command := range commands {
		if command.pattern != nil {
			if command.pattern.MatchString(path) {
				commandStr := command.pattern.ReplaceAllString(command.Command, path)
				if _, found := commandsRun[commandStr]; !found {
					commandsRun[commandStr] = command
					trace("match command %s: %s", commandName, commandStr)
				}
			}
		}
	}
}

func scanAndGetCommands(root string, commands map[string]*command) map[string]*command {
	commandsRun := map[string]*command{}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			trace("unexpected error walking file system path: %s|%s err: %s", root, path, err)
		} else if !info.IsDir() {
			matchCommands(commandsRun, path)
		}
		return nil
	})
	return commandsRun
}

func (cmd *command) forceKillProcess(cmdString string) {
	if command, ok := cmd.running[cmdString]; ok {
		processKillWait(command)
		delete(cmd.running, cmdString)
	}
}

func processKillWait(command *exec.Cmd) {
	if command.Process != nil {
		command.Process.Kill()
		command.Wait()
	}
}

func (cmd *command) forceKillAllProcess() {
	for cmdString, command := range cmd.running {
		processKillWait(command)
		delete(cmd.running, cmdString)
	}
}

// Unquote interprets s as a single-quoted, double-quoted,
// or backquoted Go string literal, returning the string value
// that s quotes.  (If s is single-quoted, it would be a Go
// character literal; Unquote returns the corresponding
// one-character string.)
func unQuote(s string) (t string, err error) {
	n := len(s)
	if n < 2 {
		return "", strconv.ErrSyntax
	}
	quote := s[0]
	if quote != s[n-1] {
		return "", strconv.ErrSyntax
	}
	s = s[1 : n-1]

	if quote == '`' {
		if contains(s, '`') {
			return "", strconv.ErrSyntax
		}
		return s, nil
	}
	if quote != '"' && quote != '\'' {
		return "", strconv.ErrSyntax
	}
	if contains(s, '\n') {
		return "", strconv.ErrSyntax
	}

	// Is it trivial?  Avoid allocation.
	if !contains(s, '\\') && !contains(s, quote) {
		switch quote {
		case '"':
			return s, nil
		case '\'':
			r, size := utf8.DecodeRuneInString(s)
			if size == len(s) && (r != utf8.RuneError || size != 1) {
				return s, nil
			}
		}
	}

	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(s)/2) // Try to avoid more allocations.
	for len(s) > 0 {
		c, multibyte, ss, err := strconv.UnquoteChar(s, quote)
		if err != nil {
			return "", err
		}
		s = ss
		if c < utf8.RuneSelf || !multibyte {
			buf = append(buf, byte(c))
		} else {
			n := utf8.EncodeRune(runeTmp[:], c)
			buf = append(buf, runeTmp[:n]...)
		}
	}
	return string(buf), nil
}

// contains reports whether the string contains the byte c.
func contains(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}
