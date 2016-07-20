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

// +build !darwin

package main

import (
	"github.com/fsnotify/fsnotify"
	"os"
	"path"
)

func trackModifications() {

	fse, err := fsnotify.NewWatcher()
	if err != nil {
		trace("error starting modifications tracker: %s", err)
		os.Exit(1)
	}
	defer fse.Close()

	go func() {
		for ev := range fse.Events {
			debug("event: %q", ev.Name)
			pendingMx.Lock()
			matchCommands(pendingCommands, path.Join(root, event.Path))
			pendingMx.Unlock()
		}
	}()

	err = fse.Add(root)
	if err != nil {
		trace("error registering the watcher path: %s", err)
	}
}
