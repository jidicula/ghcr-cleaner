package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v42/github"
	"golang.org/x/oauth2"

	flag "github.com/spf13/pflag"
)

func main() {
	token := flag.String("token", "", "GitHub personal API token with `read:packages` and `delete:packages` scope")
	packageName := flag.String("packagename", "", "Name of package")

	flag.Parse()
	if *token == "" {
		fmt.Fprintf(os.Stderr, "No token provided\n")
		os.Exit(1)
	}
	if *token == "" {
		fmt.Fprintf(os.Stderr, "No package name provided\n")
		os.Exit(1)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	opts := &github.PackageListOptions{
		PackageType: github.String("container"),
		ListOptions: github.ListOptions{
			Page:    0,
			PerPage: 100,
		},
	}
	packageList, resp, err := client.Users.PackageGetAllVersions(ctx, "", "container", *packageName, opts)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
	}
	untaggedPackageIDs := make(map[int64]github.PackageVersion)
	for _, p := range packageList {
		if len(p.GetMetadata().Container.Tags) == 0 {
			untaggedPackageIDs[p.GetID()] = *p
		}
	}

	for i := resp.NextPage; i <= resp.LastPage; i++ {
		opts = &github.PackageListOptions{
			PackageType: github.String("container"),
			ListOptions: github.ListOptions{
				Page:    i,
				PerPage: 100,
			},
		}
		pageList, resp, err := client.Users.PackageGetAllVersions(ctx, "", "container", *packageName, opts)
		errRespCheck(err, *resp)
		for _, p := range pageList {
			if len(p.GetMetadata().Container.Tags) == 0 {
				untaggedPackageIDs[p.GetID()] = *p
			}
		}
	}

	type result struct {
		sha265  string
		success bool
	}

	ch := make(chan result, len(untaggedPackageIDs))
	for id, p := range untaggedPackageIDs {
		if len(p.GetMetadata().Container.Tags) == 0 {
			func(val string) {
				resp, err := client.Users.PackageDeleteVersion(ctx, "", "container", "clang-format", id)
				res := result{
					sha265:  val,
					success: errRespCheck(err, *resp),
				}
				ch <- res
			}(p.GetName())
		}
	}

	for i := 0; i < len(untaggedPackageIDs); i++ {
		res := <-ch
		if res.success {
			fmt.Printf("Deleted container %s\n", res.sha265)
		}
	}
}

func errRespCheck(err error, resp github.Response) bool {
	ok := true
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		ok = false
	}
	if err := github.CheckResponse(resp.Response); err != nil {
		fmt.Fprintln(os.Stderr, err)
		ok = false
	}
	return ok
}
