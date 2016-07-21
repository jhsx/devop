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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Service struct {
	DevPort string
	AppPort string
	Refresh string
	Dir     string

	Env      []string
	Commands map[string]*command
}

type command struct {
	Building       string
	Match, Command string
	Continue       string
	Oninit, Onexit string
	Dir            string
	Env            []string

	Wait, Stderr, Stdout bool

	pattern *regexp.Regexp

	running map[string]*exec.Cmd
}

func (s *Service) Init() {

	if _port != nil && *_port != "" {
		ports := strings.SplitN(*_port, ":", 2)
		s.DevPort = ports[0]

		if len(ports) > 1 {
			s.AppPort = ports[1]
		}

		if s.DevPort == "" {
			s.DevPort = "8080"
		}

		if s.AppPort == "" {
			port, _ := strconv.Atoi(s.AppPort)
			s.AppPort = fmt.Sprint(port + 1)
		}
	} else if s.DevPort != "" && s.AppPort == "" {
		s.AppPort = "8080"
	}

	if _tickerDuration != nil && *_tickerDuration != "" {
		s.Refresh = *_tickerDuration
	} else if s.Refresh == "" {
		s.Refresh = "2s"
	}

	if s.Dir == "" {
		s.Dir, _ = os.Getwd()
	} else {
		s.Dir, _ = filepath.Abs(s.Dir)
	}

	s.Env = append(append([]string{}, os.Environ()...), s.Env...)
	sExpander := expander(s.Env)

	for commandName, command := range s.Commands {

		if *_debug {
			trace("loading command: %v pattern: %v cmd: %v", commandName, command.Match, command.Command)
		}

		if command.Match != "" {
			command.pattern = regexp.MustCompile(command.Match)
		}

		if !command.Wait {
			command.running = make(map[string]*exec.Cmd)
		}

		if len(command.Env) > 0 {
			for i := 0; i < len(command.Env); i++ {
				command.Env[i] = os.Expand(command.Env[i], sExpander)
			}
			command.Env = append(append(make([]string, 0, len(s.Env)), s.Env...), command.Env...)
		} else {
			command.Env = s.Env
		}

		expander := expander(command.Env)

		command.Onexit = os.Expand(command.Onexit, expander)
		command.Oninit = os.Expand(command.Oninit, expander)
		command.Command = os.Expand(command.Command, expander)

		if command.Dir != "" {
			command.Dir = os.Expand(command.Dir, expander)
		}

		if command.Oninit != "" {
			trace("Running init command for %s: %q", commandName, command.Oninit)
			cmd := newProcessCommand(command.Oninit)
			cmd.Env = command.Env
			err := cmd.Run()
			if err != nil {
				trace("err:%s", err)
				return
			}
		}
	}
}

func expander(envs []string) func(string) string {
	return func(s string) (found string) {
		for _, v := range envs {
			env := strings.SplitN(v, "=", 2)
			if env[0] == s {
				found = env[1]
			}
		}
		return
	}
}

func (s *Service) GetRoot() string {
	return s.Dir
}

func (s *Service) GetEnv() []string {
	return s.Env
}
