package change 

import (
	"bytes"
	"strings"
	"fmt"
	"regexp"
	"context"
	"net/url"
	"gerrit-mcp/internal/logger"
	"gerrit-mcp/internal/util"
	"strconv"
	"github.com/andygrunwald/go-gerrit"
)

const (
	FileDiffsLimit = 3 
)

type GerritChange struct {
	// GerritInfo     gerrit.ChangeInfo
	// GerritDiffInfo gerrit.DiffInfo
	Paths         []string
	Subject       string
	Type          string
	Project       string
	URL           string
	DiffSample    []byte
	DiffMap       map[string]string
	IsInteresting bool
}

func NewGerritChange(changeInfo *gerrit.ChangeInfo, diffsInfo []*gerrit.DiffInfo, endpointURL string) (GerritChange, error) {
	fpaths := make([]string, 0)
	diffMap := make(map[string]string, 0)
	for index, diffInfo := range diffsInfo {
		content := diffInfo.Content
		switch diffInfo.ChangeType {
		case "ADDED":
			fpaths = append(fpaths, diffInfo.MetaB.Name)
			if index > FileDiffsLimit {
				continue
			}
			var buf bytes.Buffer
			for _, data := range content {
				buf.WriteString(strings.Join(data.B, "\r\n"))
			}
			diffMap[diffInfo.MetaB.Name] = buf.String()
		case "MODIFIED":
			if index > FileDiffsLimit {
				continue
			}
			fpaths = append(fpaths, diffInfo.MetaA.Name)
			var buf bytes.Buffer
			for _, data := range content {
				buf.WriteString(strings.Join(data.B, "\r\n"))
			}
			diffMap[diffInfo.MetaB.Name] = buf.String()
		}
	}

	return GerritChange{Paths: fpaths, Type: "dummy",
		Subject: changeInfo.Subject, Project: changeInfo.Project,
		DiffMap: diffMap,
		URL:     extractChangeId(changeInfo.ChangeID, endpointURL)}, nil
}

func BuildGerritChanges(ctx context.Context, gerritClient *gerrit.Client, changes *[]gerrit.ChangeInfo) ([]GerritChange, error) {
	gerritChanges := make([]GerritChange, 0)
	for _, curChange := range *changes {
		logger.Debugf("processing %s %s", curChange.ID, curChange.Subject)
		revision := curChange.CurrentRevision
		if revision == "" {
			revision = "current"
		}
		unfilteredFiles, _, rerr := gerritClient.Changes.ListFiles(ctx, curChange.ID, revision, &gerrit.FilesOptions{})
		if rerr != nil {
			// panic(rerr)
			logger.Errorf("%v", rerr)
			continue
		}
		files := util.FilterFiles(unfilteredFiles)
		logger.Debugf("filtered files count %d\n", len(files))
		// TODO: move to ShouldSkipChange
		if len(files) > 32 || len(files) == 0 {
			continue
		}
		logger.Debugf("moving with %s\n", strings.Join(files, "\n"))
		diffs := make([]*gerrit.DiffInfo, 0)
		for _, fname := range files {
			if fname == "/COMMIT_MSG" || fname == "/MERGE_LIST" || fname == "/PATCHSET_LEVEL" {
				continue
			}
			diffInfo, _, diffErr := gerritClient.Changes.GetDiff(ctx, curChange.ID, revision, fname, nil)
			if diffErr != nil {
				logger.Errorf("%v", diffErr)
			}
			diffs = append(diffs, diffInfo)
		}
		u := gerritClient.BaseURL()
		gerritChange, err := NewGerritChange(&curChange, diffs, u.String())
		if err != nil {
			logger.Errorf("%v", err)
		}
		gerritChanges = append(gerritChanges, gerritChange)
	}
	return gerritChanges, nil
}

func extractChangeId(rawChangeId string, endpointURL string) string {
	regexp, err := regexp.Compile(`[A-Za-z0-9]{32,}`)
	if err != nil {
		return rawChangeId
	}
	changeId := regexp.FindString(rawChangeId)
	if changeId != "" {
		return fmt.Sprintf("%s/q/%s", endpointURL, changeId)
	}
	return rawChangeId
}

// example: https://chromium-review.googlesource.com/c/chromium/src/+/4640000
func BuildQueryFromURL(reviewURL string) (string, error) {
	// opt := &gerrit.QueryChangeOptions{}
	u, err := url.Parse(reviewURL)
	if err != nil {
		return "", err
	}
	path := u.EscapedPath()
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid review URL: %s (parts: %v)", reviewURL, parts)
	}
	urlType := parts[1]
	switch urlType {
	case "q":
		logger.Debugf("query change URL: %s", reviewURL)
		changeID := parts[2]
		return fmt.Sprintf("change:%s", changeID), nil
		// opt.Query = []string{fmt.Sprintf("change:%s", changeID)}
	case "c":
		if len(parts) < 5 {
			return "", fmt.Errorf("invalid review URL: %s (parts: %v)", reviewURL, parts)
		}
		if parts[1] != "c" || parts[4] != "+" {
			return "", fmt.Errorf("invalid review URL: %s (parts: %v)", reviewURL, parts)
		}
		// project := parts[2]
		// branch := parts[3]
		changeNumber := parts[5]
		changeNumberInt, err := strconv.Atoi(changeNumber)
		if err != nil {
			return "", fmt.Errorf("invalid change number: %s", changeNumber)
		}
		return fmt.Sprintf("change:%d", changeNumberInt), nil
		// opt.Query = []string{fmt.Sprintf("change:%d", parts[5])}
	}
	return "", fmt.Errorf("invalid review URL: %s (parts: %v)", reviewURL, parts)
}
