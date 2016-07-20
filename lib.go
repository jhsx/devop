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
	} else {
		if s.AppPort == "" {
			s.AppPort = "8080"
		}
		if s.DevPort == "" {
			s.DevPort = "8888"
		}
	}

	if _tickerDuration != nil && *_tickerDuration != "" {
		s.Refresh = *_tickerDuration
	} else if s.Refresh == "" {
		s.Refresh = "2s"
	}
}

func (s *Service) GetRoot() string {
	if s.Dir == "" {
		s.Dir, _ = os.Getwd()
	}
	return s.Dir

}

func (s *Service) GetEnv() []string {
	return append(os.Environ(), s.Env...)
}
