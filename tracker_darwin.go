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
	"github.com/fsnotify/fsevents"
	"time"
)

func trackModifications() {

	fse := &fsevents.EventStream{
		Paths:   []string{root},
		Latency: 500 * time.Millisecond,
		// Device:  dev,
		Flags: fsevents.FileEvents | fsevents.WatchRoot,
	}

	fse.Start()

	for ev := range fse.Events {
		for _, event := range ev {
			if event.Flags&fsevents.ItemIsFile == fsevents.ItemIsFile {
				switch {
				case event.Flags&fsevents.ItemCreated == fsevents.ItemCreated,
					event.Flags&fsevents.ItemModified == fsevents.ItemModified,
					event.Flags&fsevents.ItemRemoved == fsevents.ItemRemoved,
					event.Flags&fsevents.ItemRenamed == fsevents.ItemRenamed:
					debug("event: %q", event.Path)

					pendingMx.Lock()
					matchCommands(pendingCommands, event.Path)
					pendingMx.Unlock()

				}
			}
		}
	}

}
