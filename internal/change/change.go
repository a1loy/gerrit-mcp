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
	Messages []string
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

	changeMessages := make([]string, 0)
	for _, message := range changeInfo.Messages {
		changeMessages = append(changeMessages, fmt.Sprintf("%s: %s", message.Author.Name, message.Message))
	}

	return GerritChange{Paths: fpaths, Type: "dummy",
		Subject: changeInfo.Subject, Project: changeInfo.Project,
		DiffMap: diffMap,
		Messages: changeMessages,
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

func (c * GerritChange) TextResult() string {
	resultBuilder := strings.Builder{}
	resultBuilder.WriteString(fmt.Sprintf("%s: %s\n", c.URL, c.Subject))
	resultBuilder.WriteString(fmt.Sprintf("Changed files: %s\n", strings.Join(c.Paths, "\n")))
	for fname, diff := range c.DiffMap {
		resultBuilder.WriteString(fmt.Sprintf("%s:\n%s\n", fname, diff))
	}
	// for _, message := range c.Messages {
	// 	resultBuilder.WriteString(fmt.Sprintf("%s\n", message))
	// }
	return resultBuilder.String()
}

func GetCorrectProjectName(ctx context.Context, gerritClient *gerrit.Client, projectName string, defaultProject string) string {
	// TODO: query project from gerrit instance
	projectMapping := map[string]string{
		"chromium": "chromium/src",
		"v8": "v8/v8",
	}
	knownProjects := make([]string, 0)
	opt := &gerrit.ProjectOptions{}
	projects, _, err := gerritClient.Projects.ListProjects(ctx, opt)
	if err == nil {
		for _, p := range *projects {
			knownProjects = append(knownProjects, p.Name)
		}
	}
	for _, p := range knownProjects {
		if strings.HasPrefix(p, projectName) {
			return p
		}
	}
	for k, v := range projectMapping {
		if strings.HasPrefix(k, projectName) || strings.HasPrefix(v, projectName) {
			return v
		}
	}
	return defaultProject
}