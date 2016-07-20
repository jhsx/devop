# DevOp Dev Server

Devop is a development server for Go, in devop you can declare commands that will be runned when any file system modification happens.

Example you can declare a command to run "go build ." on every modification in the file system that match the file name that end with .go.


## File system modifications:

File changed, renamed, removed or created.


Devop lets you program in go as you do with PHP, just refresh the page and you get the last changes.

example yml file:

```yaml
devPort: 8888 # the port used by the proxy server
appPort: 8080 # the port used by your app
commands: #your commands
  gobuild: #command name
    match: "\\.go$" # regex pattern, this pattern is tested on all modified files
    command: "$HOME/tipgo/go/bin/go build -i -gcflags=-N" # the command that need to be executed when a pattern match on modifications
    wait: true # need to wait this command to finish before continue with the next command
    stderr: true # want to print the stderr
    stdout: true # want to print the stdout
    continue: gorun # when this command finish to run continue with command "gorun"
  gorun: # this command don't have a match, so the command will only run when an other command say's continue: to this command name
    command: ./yourapp
    env: # list of env
      - MONGOSERVER=localhost
      - MONGODB=test
      - PORT=:8080
      - MODE=DEV
    stderr: true
    stdout: true
    onexit: rm ekart-serve # command to be executed when devop exits
```
