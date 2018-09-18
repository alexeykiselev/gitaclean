# gitaclean

gitaclean - a simple tool to remove the tags that does not have releases associated with them.

## Installation
Build for yourself or download desired binary from the releases page. Rename the binary to `gitaclean` and add an execution flag if needed.

## Usage
```bash
gitaclean
  -o | -owner name    repository owner name
  -r | -repo  name    repositor name
  -t | -token string  github application token
  -dry-run            print tags that will be removed
  -v | -version       print version
```
To obtain the token follow [the instructions](https://developer.github.com/apps/building-github-apps/authenticating-with-github-apps/).

## Cleaning local Git repo

```bash
git fetch --prune origin "+refs/tags/*:refs/tags/*"
```