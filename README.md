# fsync - synchronize local files to a remote host

fsync is a utility which watches the provided directory for changes and
mirrors said changes to the remote server. Currently the following operations
are supported:

- Renaming (seen as a delete then create)
- Creating directories
- Creating files
- Modifying files
- Deleting files or directories

Some notes:

- Modifying or deleting files outside of the provided remote root directory
is strictly disallowed. This is to prevent damage to your system in case there's
a bug in this application
- Deleting the remote root directory is strictly disallowed for the same reason
- There are probably bugs. At the time of writing there are 0 unit test

## Installation

### Requirements

- Go >= 1.5
- [Glide](http://glide.sh/)

### Manual Install

```
cd $GOPATH/src
mkdir -p gitlab.com/landaire/fsync
git clone https://gitlab.com/landaire/fsync.git
glide install
go install gitlab.com/landaire/fsync
```

This is a little more painful than a normal Go application install, but that's
because the `vendor/` directory is not committed and `glide` doesn't offer
an `install` command similar to `go install` which:

1.) Fetches the repo
2.) Fetches and installs dependencies
3.) Installs the application

## Usage

Usage is fairly straightforward. At the time of writing only public key-based
authentication is supported, but interactive auth will be added in the near
future (this is simple, just requires actually doing it).

To watch changes from the current working directory:

`fsync -i ~/.ssh/your_key.pem user@host:/root/directory`

This is equivalent to:

`fsync -i ~/.ssh/your_key.pem . user@host:/root/directory`
