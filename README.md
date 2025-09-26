# cody
> Code repository management

### What is Cody?

**Cody** is a command line utilitity program that helps get various git projects organized and easy to find. The `code.d` directory in your home folder contains the git urls and acts as an index for your projects.

```
# ~/.code.d/github.com
git@github.com:alexjpaz/cody.git
```

Cody can then run various tasks for these repositories

* `pull [pattern]` - pull down all matching repositories
* `search [pattern]` - find the directory the repository is in (see [shell integration](#shell-integration))
* `open [pattern]` - open the git repository in the browser (useful for github.com repositories)

### Installation

Download the [latest release binary](https://github.com/alexjpaz/cody/releases) for your OS.

Copy the binary to a directory in your `$PATH` (e.g. /usr/local/bin)

You will need to add some git repo urls to the `~/.code.d` directory.

```
mkdir -o ~/.code.d/
echo "git@github.com:alexjpaz/cody.git" >> ~/.code.d/github.code
```

```
cody add github git@github.com:alexjpaz/cody.git
```

#### Usage

Pull all repositories

```
cody pull
```

Search for a repository and print the directory

```
cody cody
```

### Shell Integration

In order to get the benefit of some features in cody you will need to copy the following into your shell configuration


#### Bash

```
function cody_cd() {
    eval $(cody open $@)
    cd $(cat /tmp/cody_result)
}
```
