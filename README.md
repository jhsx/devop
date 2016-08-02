# DevOp Dev Server [![Build Status](https://travis-ci.org/jhsx/devop.svg?branch=master)](https://travis-ci.org/jhsx/devop) [![Build status](https://ci.appveyor.com/api/projects/status/eun8c8h8x4u4gnnw?svg=true)](https://ci.appveyor.com/project/jhsx/devop)

Devop is a development server for Go, in devop you can declare commands that will be runned when any file system modification happens.

Example you can declare a command to run "go build ." on every modification in the file system that match the file name that end with .go.


## File system modifications:

File changed, renamed, removed or created.


Devop lets you program in go as you do with PHP, just refresh the page and you get the last changes.

## How to use

First install devop with
```
go get github.com/jhsx/devop
```
Create an devop.yml in the root of your project, then run devop and enjoy your programing time :D.

Example devop.yml file:

```yaml
devPort: 8888 # the port used by the proxy server
appPort: 8080 # the port used by your app
commands: #your commands
  gobuild: #command name
    match: "\\.go$" # regex pattern, this pattern is tested on all modified files
    command: "go build -i -gcflags='-N -l'" # the command that need to be executed when a pattern matched on modifications
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
    onexit: rm yourapp # command to be executed when devop exits
```
