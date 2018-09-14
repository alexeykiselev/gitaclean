package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"io"
	"io/ioutil"
	"os"
)

type Cleaner struct {
	ctx        context.Context
	client     *github.Client
	owner      string
	repository string
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

func (c *Cleaner) clone(repo github.Repository) (*git.Repository, error) {
	p, err := ioutil.TempDir("", "gitaclean")
	if err != nil {
		return nil, err
	}
	return git.PlainClone(p,  true, &git.CloneOptions{URL: *repo.CloneURL})
}

func (c *Cleaner) remove(repo *git.Repository, tags []*github.RepositoryTag) error {
	tagObjectsIter, err := repo.TagObjects()
	if err != nil {
		return err
	}
	fmt.Println("CHECK")
	fmt.Println("TAG OBJECTS:")
	for _, to := range c.listTagObjects(repo) {
		fmt.Println(to)
	}
	fmt.Println("TAG REFS:")
	for _, tr := range c.listTagRefs(repo) {
		fmt.Println(tr)
	}
	fmt.Println("-= Removing tag objects =-")
	tagObj, e := tagObjectsIter.Next()
	for e == nil || e != io.EOF {
		for _, tt := range tags {
			if tagObj.Name == *tt.Name {
				rn := plumbing.ReferenceName("refs/tags/" + tagObj.Name)
				fmt.Println("ID: ", tagObj.ID(), "Name:", tagObj.Name, "HASH:", tagObj.Hash, "TARGET:", tagObj.Target.String(), tagObj.TargetType.String())
				to, err := repo.TagObject(tagObj.ID())
				if err != nil {
					fmt.Println("TO FAIL", err.Error())
				}
				repo.o
				o := to.Decode()
				err := repo.DeleteObject(tagObj.ID())
				if err != nil {
					fmt.Printf("Failed to delete tag object '%s': %s\n", tagObj.Name, err.Error())
				} else {
					err := repo.Storer.RemoveReference(rn)
					if err != nil {
						fmt.Printf("Failed to remove tag '%s': %s\n", tagObj.Name, err.Error())
					} else {
						fmt.Printf("Tag '%s' successfuly removed\n", tagObj.Name)
					}
				}
				break
			}
		}
		tagObj, e = tagObjectsIter.Next()
	}
	tagsIter, err := repo.Tags()
	if err != nil {
		return err
	}
	fmt.Println("-= Removing tag references =-")
	tag, e := tagsIter.Next()
	for e == nil || e != io.EOF {
		for _, tt := range tags {
			if tag.Name().IsTag() && tag.Name().Short() == *tt.Name {
				err := repo.Storer.RemoveReference(plumbing.ReferenceName(tag.Name().String()))
				if err != nil {
					fmt.Printf("Failed to remove tag '%s': %s\n", tag.Name().Short(), err.Error())
				} else {
					fmt.Printf("Tag '%s' successfuly removed\n", tag.Name().Short())
				}
				break
			}
		}
		tag, e = tagsIter.Next()
	}
	fmt.Println("CHECK")
	fmt.Println("TAG OBJECTS:")
	for _, t := range c.listTagObjects(repo) {
		fmt.Println(t)
	}
	fmt.Println("TAG REFS:")
	for _, t := range c.listTagRefs(repo) {
		fmt.Println(t)
	}
	return nil
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
		clone, err := cleaner.clone(*repo)
		if err != nil {
			fmt.Println("Failed to clone repository:", err.Error())
			os.Exit(1)
		}
		fmt.Println("Done")
		fmt.Printf("Removing %d unreleased tags...\n", len(unreleased))
		err = cleaner.remove(clone, unreleased)
		if err != nil {
			fmt.Printf("Failed to remove unreleased tags")
			os.Exit(1)
		}
	} else {
		for _, u := range unreleased {
			fmt.Printf("DRY-RUN: Tag '%s' was removed\n", *u.Name)
		}
	}

}
