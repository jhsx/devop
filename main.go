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
	"regexp"
	"sync"
	"syscall"
	"time"
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

	environ := devService.GetEnv()
	root = devService.GetRoot()
	commands = devService.Commands

	for commandName, command := range commands {
		if *_debug {
			trace("loading command: %v pattern: %v cmd: %v", commandName, command.Match, command.Command)
		}

		if command.Match != "" {
			command.pattern = regexp.MustCompile(command.Match)
		}

		if command.Dir != "" {
			command.Dir = os.ExpandEnv(command.Dir)
		}

		if !command.Wait {
			command.running = make(map[string]*exec.Cmd)
		}
		if len(command.Env) > 0 {
			for i := 0; i < len(command.Env); i++ {
				command.Env[i] = os.ExpandEnv(command.Env[i])
			}
			command.Env = append(append(make([]string, 0, len(environ)), environ...), command.Env...)
		}

		if command.Oninit != "" {
			trace("Running init command for %s: %q", commandName, command.Oninit)
			cmd := exec.Command("bash", "-c", command.Oninit)
			cmd.Env = command.Env
			err = cmd.Run()
			if err != nil {
				trace("err:%s", err)
				return
			}
		}
	}
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
				cmd := exec.Command("bash", "-c", command.Onexit)
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
	autoRefresh()

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
}

func autoRefresh() {
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

	cmd := exec.Command("bash", "-c", cmdString)
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
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
		if !info.IsDir() {
			matchCommands(commandsRun, path)
		}
		return nil
	})
	return commandsRun
}

func (cmd *command) forceKillProcess(cmdString string) {
	if command, ok := cmd.running[cmdString]; ok {
		killAProcessGroup(command)
		delete(cmd.running, cmdString)
	}
}

func killAProcessGroup(command *exec.Cmd) {
	pid, _ := syscall.Getpgid(command.Process.Pid)
	syscall.Kill(-pid, os.Kill.(syscall.Signal))
	command.Process.Signal(os.Kill)
	command.Wait()
}

func (cmd *command) forceKillAllProcess() {
	for cmdString, command := range cmd.running {
		killAProcessGroup(command)
		delete(cmd.running, cmdString)
	}
}