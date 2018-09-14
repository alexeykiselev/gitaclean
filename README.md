# gitaclean

gitaclean - a simple tool to remove the tags that does not have associated releases.

## Usage
```
gitaclean
  -o | -owner name    repository owner name
  -r | -repo  name    repositor name
  -t | -token string  github application token
  -dry-run            print tags that will be removed
```
To obtain the token follow [the instructions](https://developer.github.com/apps/building-github-apps/authenticating-with-github-apps/).
