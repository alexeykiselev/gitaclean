package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"io"
	"io/ioutil"
	"os"
)

type Cleaner struct {
	ctx        context.Context
	client     *github.Client
	owner      string
	repository string
	token      string
}

func NewCleaner(token, owner, repository string) *Cleaner {
	ctx := context.Background()
	t := oauth2.Token{AccessToken: token}
	ts := oauth2.StaticTokenSource(&t)
	tc := oauth2.NewClient(ctx, ts)
	cl := github.NewClient(tc)
	return &Cleaner{
		ctx:        ctx,
		client:     cl,
		owner:      owner,
		repository: repository,
		token:      token,
	}
}

func (c *Cleaner) releases() ([]*github.RepositoryRelease, error) {
	opt := &github.ListOptions{}
	var r []*github.RepositoryRelease
	for {
		rs, rsp, err := c.client.Repositories.ListReleases(c.ctx, c.owner, c.repository, opt)
		if err != nil {
			return nil, err
		}
		r = append(r, rs...)
		if rsp.NextPage == 0 {
			break
		}
		opt.Page = rsp.NextPage
	}
	return r, nil
}

func (c *Cleaner) tags() ([]*github.RepositoryTag, error) {
	opt := &github.ListOptions{}
	var r []*github.RepositoryTag
	for {
		ts, rsp, err := c.client.Repositories.ListTags(c.ctx, c.owner, c.repository, opt)
		if err != nil {
			return nil, err
		}
		r = append(r, ts...)
		if rsp.NextPage == 0 {
			break
		}
		opt.Page = rsp.NextPage
	}
	return r, nil
}

func (c *Cleaner) repo() (*github.Repository, error) {
	r, _, err := c.client.Repositories.Get(c.ctx, c.owner, c.repository)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (c *Cleaner) clone(repo github.Repository) (*git.Repository, string, error) {
	p, err := ioutil.TempDir("", "gitaclean")
	if err != nil {
		return nil, "", err
	}
	r, err := git.PlainClone(p, true, &git.CloneOptions{URL: *repo.CloneURL})
	rem, _ := r.Remote("origin")
	fmt.Println("REMOTE:", rem)
	fmt.Println("OPTS:", rem.Config())
	return r, p, err
}

func (c *Cleaner) remove(repo *git.Repository, tags []*github.RepositoryTag) ([]*github.RepositoryTag, []*github.RepositoryTag, error) {
	iter, err := repo.Tags()
	if err != nil {
		return nil, tags, err
	}
	var s []*github.RepositoryTag
	var f []*github.RepositoryTag
	tag, e := iter.Next()
	for e == nil || e != io.EOF {
		for _, tt := range tags {
			if tag.Name().IsTag() && tag.Name().Short() == *tt.Name {
				err := repo.Storer.RemoveReference(plumbing.ReferenceName(tag.Name().String()))
				if err != nil {
					f = append(f, tt)
				} else {
					s = append(s, tt)
				}
				break
			}
		}
		tag, e = iter.Next()
	}
	return s, f, nil
}

func (c *Cleaner) listTagObjects(repo *git.Repository) []string {
	var r []string
	iter, err := repo.TagObjects()
	if err == nil {
		t, e := iter.Next()
		for e == nil || e != io.EOF {
			r = append(r, t.Name)
			t, e = iter.Next()
		}
	}
	return r
}

func (c *Cleaner) listTagRefs(repo *git.Repository) []string {
	var r []string
	iter, err := repo.Tags()
	if err == nil {
		t, e := iter.Next()
		for e == nil || e != io.EOF {
			if t.Name().IsTag() {
				r = append(r, t.Name().Short())
			}
			t, e = iter.Next()
		}
	}
	return r
}

func (c *Cleaner) Push(repo *git.Repository, refs []config.RefSpec) error {
	auth := http.BasicAuth{Username: c.owner, Password: c.token}
	o := git.PushOptions{RemoteName: "origin", Auth: &auth, RefSpecs: refs}
	return repo.Push(&o)
}

var defaultOwner = "wavesplatform"
var defaultRepository = "Waves"

func main() {
	var owner string
	var repository string
	var token string
	var dry bool

	flag.StringVar(&owner, "o", defaultOwner, "repository owner name")
	flag.StringVar(&owner, "owner", defaultOwner, "repository owner name")
	flag.StringVar(&repository, "r", defaultRepository, "repository name")
	flag.StringVar(&repository, "repo", defaultRepository, "repository name")
	flag.StringVar(&token, "t", "", "GitHub application token")
	flag.StringVar(&token, "token", "", "GitHub application token")
	flag.BoolVar(&dry, "dry-run", false, "print tags that will be removed")
	flag.Parse()

	if token == "" {
		fmt.Println("Empty GitHub application token")
		os.Exit(1)
	}

	cleaner := NewCleaner(token, owner, repository)

	fmt.Println("Requesting repository info...")
	repo, err := cleaner.repo()
	if err != nil {
		fmt.Println("Failed to get repository:", err.Error())
		os.Exit(1)
	}
	fmt.Printf("Info about '%s' repository received\n", *repo.Name)
	fmt.Println("Requesting releases... ")
	releases, err := cleaner.releases()
	if err != nil {
		fmt.Println("Failed to get all releases:", err.Error())
		os.Exit(1)
	}
	fmt.Printf("Received %d releases\n", len(releases))
	fmt.Println("Requesting tags...")
	tags, err := cleaner.tags()
	if err != nil {
		fmt.Println("Failed to get all tags:", err.Error())
		os.Exit(1)
	}
	fmt.Printf("Received %d tags\n", len(tags))

	var unreleased []*github.RepositoryTag
	for _, t := range tags {
		uf := true
		for _, r := range releases {
			if *t.Name == *r.TagName {
				uf = false
				break
			}
		}
		if uf {
			unreleased = append(unreleased, t)
		}
	}

	if !dry {
		fmt.Printf("Cloning repository '%s'...\n", *repo.Name)
		clone, dir, err := cleaner.clone(*repo)
		defer os.RemoveAll(dir)
		if err != nil {
			fmt.Println("Failed to clone repository:", err.Error())
			os.Exit(1)
		}
		fmt.Println("Done")
		fmt.Printf("Removing %d unreleased tags...\n", len(unreleased))
		sr, fr, err := cleaner.remove(clone, unreleased)
		if err != nil {
			fmt.Printf("Failed to remove unreleased tags\n")
			os.Exit(1)
		}
		var refs []config.RefSpec
		for _, t := range sr {
			fmt.Printf("Successfully removed tag '%s'\n", *t.Name)
			refs = append(refs, config.RefSpec(fmt.Sprintf(":refs/tags/%s", *t.Name)))
		}
		for _, t := range fr {
			fmt.Printf("Failed to remove tag '%s'\n", *t.Name)
		}
		fmt.Println("Pushing repository...")
		fmt.Println("REFS:", refs)
		err = cleaner.Push(clone, refs)
		if err != nil {
			fmt.Printf("Failed to push repository: %s\n", err)
		} else {
			fmt.Println("Done")
		}
	} else {
		for _, u := range unreleased {
			fmt.Printf("Tag '%s' will be removed\n", *u.Name)
		}
	}

}
